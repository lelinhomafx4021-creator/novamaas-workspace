package model

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type snapshotLookupSQLRecorder struct {
	statements []string
}

func (recorder *snapshotLookupSQLRecorder) LogMode(logger.LogLevel) logger.Interface { return recorder }
func (recorder *snapshotLookupSQLRecorder) Info(context.Context, string, ...any)     {}
func (recorder *snapshotLookupSQLRecorder) Warn(context.Context, string, ...any)     {}
func (recorder *snapshotLookupSQLRecorder) Error(context.Context, string, ...any)    {}
func (recorder *snapshotLookupSQLRecorder) Trace(_ context.Context, _ time.Time, sql func() (string, int64), _ error) {
	statement, _ := sql()
	recorder.statements = append(recorder.statements, statement)
}

func TestSnapshotLookupReadsOnlySelectedInlineSourceAndSkipsReadyBody(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	largeBody := []byte(`{"input":"` + strings.Repeat("A", 3*1024*1024) + `"}`)
	taskUpstreamRef, err := upstreamRequestBodyReference("source-task", "")
	require.NoError(t, err)
	requestUpstreamRef, err := upstreamRequestBodyReference("", "source-request")
	require.NoError(t, err)
	require.NoError(t, db.Create(&[]TaskRequestBody{
		{ReferenceID: "source-task", TaskID: "source-task", Kind: TaskRequestSnapshotOriginal, Body: largeBody},
		{ReferenceID: taskUpstreamRef, TaskID: "source-task", Kind: TaskRequestSnapshotUpstream, Body: largeBody},
		{ReferenceID: "request:source-request", TaskID: "source-task", RequestID: "source-request", Kind: TaskRequestSnapshotOriginal, Body: []byte(`{ "source": "request" }`)},
		{ReferenceID: requestUpstreamRef, TaskID: "source-task", RequestID: "source-request", Kind: TaskRequestSnapshotUpstream,
			Body: largeBody, StorageState: TaskRequestSnapshotReady, StorageProfileID: 7, ObjectKey: "snapshots/upstream.json"},
	}).Error)
	var requestOriginal TaskRequestBody
	require.NoError(t, db.Omit("Body").Where("reference_id = ?", "request:source-request").First(&requestOriginal).Error)
	recorder := &snapshotLookupSQLRecorder{}
	DB = db.Session(&gorm.Session{Logger: recorder})
	records, err := GetSnapshotRecords("source-task", "source-request")
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, `{ "source": "request" }`, string(records[0].Body))
	assert.Equal(t, "snapshots/upstream.json", records[1].ObjectKey)
	assert.Empty(t, records[1].Body, "a ready object must not fetch its retained migration copy")
	require.Len(t, recorder.statements, 2)
	assert.NotContains(t, strings.ToLower(recorder.statements[0]), "order by", "unique references do not require sorting")
	assert.Contains(t, recorder.statements[1], fmt.Sprintf("id IN (%d)", requestOriginal.ID), "only the selected legacy source needs its body")
	assert.NotContains(t, strings.ToLower(recorder.statements[1]), "order by", "large inline JSON is fetched through primary keys without sorting")

	recorder.statements = nil
	taskRecords, err := GetSnapshotRecords("source-task", "", true)
	require.NoError(t, err)
	require.Len(t, taskRecords, 2)
	assert.Equal(t, "source-task", taskRecords[0].ReferenceID)
	assert.Equal(t, taskUpstreamRef, taskRecords[1].ReferenceID)
	assert.Empty(t, taskRecords[0].Body)
	assert.Empty(t, taskRecords[1].Body)
	require.Len(t, recorder.statements, 1, "metadata requests must not fetch any body or task properties")
}

func TestSnapshotLookupRecoversDeletedTaskTraceAndBoundsLegacyFallback(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	largeBody := []byte(`{"input":"` + strings.Repeat("A", 3*1024*1024) + `"}`)
	require.NoError(t, db.Create(&[]TaskRequestBody{
		{ReferenceID: "upstream:request:old-request", TaskID: "deleted-task", RequestID: "old-request", Body: largeBody},
		{ReferenceID: "request:new-request", RequestID: "new-request", Body: []byte(`{"original":true}`)},
		{ReferenceID: "upstream:request:new-request", TaskID: "deleted-task", RequestID: "new-request", Body: []byte(`{"upstream":true}`)},
		{ReferenceID: taskPollHistoryPrefix + "latest", TaskID: "deleted-task", RequestID: "poll-request", Body: largeBody},
	}).Error)
	recorder := &snapshotLookupSQLRecorder{}
	DB = db.Session(&gorm.Session{Logger: recorder})
	records, err := GetSnapshotRecords("deleted-task", "")
	require.NoError(t, err, "no task table is needed to recover a failed or deleted task's linkage")
	require.Len(t, records, 2)
	assert.Equal(t, "request:new-request", records[0].ReferenceID)
	assert.Equal(t, `{"original":true}`, string(records[0].Body))
	assert.Equal(t, "upstream:request:new-request", records[1].ReferenceID)
	assert.Equal(t, `{"upstream":true}`, string(records[1].Body))
	orderedQueries := 0
	for _, statement := range recorder.statements {
		statement = strings.ToLower(statement)
		if !strings.Contains(statement, "order by") {
			continue
		}
		orderedQueries++
		assert.Contains(t, statement, "limit 1", "each task fallback returns only its newest metadata candidate")
		projection := strings.SplitN(statement, " from ", 2)[0]
		assert.NotContains(t, projection, "`body`", "sorting raw JSON reproduces MySQL error 1038")
		assert.NotContains(t, projection, "select *")
	}
	assert.Equal(t, 2, orderedQueries, "original and upstream use independent indexed fallback lookups")
}

func TestSnapshotLookupExtractsTaskRequestLinkageWithoutLoadingLegacyProperties(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	task := Task{TaskID: "property-task", Properties: Properties{
		RequestID: "property-request", RequestBody: []byte(`{"legacy":"` + strings.Repeat("A", 3*1024*1024) + `"}`),
	}}
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, SaveTaskRequestBody("", "property-request", []byte(`{"from":"request"}`)))
	recorder := &snapshotLookupSQLRecorder{}
	DB = db.Session(&gorm.Session{Logger: recorder})
	records, err := GetSnapshotRecords("property-task", "")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, `{"from":"request"}`, string(records[0].Body))
	taskQueries := 0
	for _, statement := range recorder.statements {
		if strings.Contains(statement, "FROM `tasks`") {
			taskQueries++
			assert.Contains(t, statement, "json_extract(properties, '$.request_id')", "only small task linkage may be transferred")
			assert.NotContains(t, statement, "SELECT `properties`")
		}
	}
	assert.Equal(t, 1, taskQueries)

	recorder.statements = nil
	records, err = GetSnapshotRecords("property-task", "", true)
	require.NoError(t, err)
	assert.Empty(t, records, "metadata-only lookup keeps its existing no-properties contract")
	for _, statement := range recorder.statements {
		assert.NotContains(t, statement, "FROM `tasks`")
	}
}

func TestSnapshotLookupRefreshesPublicationBetweenMetadataAndInlineRead(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, SaveTaskRequestSnapshots("publication-task", "", []byte(`{"original":true}`), []byte(`{"upstream":true}`)))
	published := false
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test_publish_after_metadata", func(tx *gorm.DB) {
		if published || tx.Statement.Table != "task_request_bodies" || !strings.Contains(tx.Statement.SQL.String(), "reference_id IN") {
			return
		}
		published = true
		tx.AddError(db.Model(&TaskRequestBody{}).Where("reference_id = ?", "publication-task").Updates(map[string]any{
			"storage_state": TaskRequestSnapshotReady, "storage_profile_id": 7, "object_key": "snapshots/published.json",
		}).Error)
	}))
	records, err := GetSnapshotRecords("publication-task", "")
	require.NoError(t, err)
	require.True(t, published)
	require.Len(t, records, 2)
	assert.Equal(t, "snapshots/published.json", records[0].ObjectKey)
	assert.Empty(t, records[0].Body, "an object published after metadata selection must not return stale inline data")
	assert.Equal(t, `{"upstream":true}`, string(records[1].Body))
}

func TestSnapshotDatabaseOperationsHonorArchiveCancellation(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&StorageProfile{}, &StorageCredential{}, &StorageObject{}))
	body := []byte(`{"content":[{"type":"text"}]}`)
	require.NoError(t, SaveTaskRequestBody("cancel-task", "cancel-request", body))
	var record TaskRequestBody
	require.NoError(t, db.First(&record).Error)
	profile := StorageProfile{Name: "cancel-profile"}
	require.NoError(t, db.Create(&profile).Error)
	credential := StorageCredential{StorageProfileID: profile.ID, Status: StorageCredentialStatusActive}
	require.NoError(t, db.Create(&credential).Error)
	object := StorageObject{StorageProfileID: profile.ID, ObjectKey: "snapshots/cancel.json", Status: StorageObjectStatusUploading}
	require.NoError(t, db.Create(&object).Error)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := GetSnapshotRecordsContext(ctx, "cancel-task", "cancel-request")
	require.ErrorIs(t, err, context.Canceled)
	_, err = FindDatabaseSnapshotBatch(0, 10, ctx)
	require.ErrorIs(t, err, context.Canceled)
	_, err = GetDatabaseSnapshotBody(record.ID, ctx)
	require.ErrorIs(t, err, context.Canceled)
	_, err = GetDatabaseSnapshotArchiveUpperID(ctx)
	require.ErrorIs(t, err, context.Canceled)
	_, err = ClearMigratedSnapshotBody(record.ID, record.BodySHA256, ctx)
	require.ErrorIs(t, err, context.Canceled)
	err = SaveExternalTaskRequestSnapshotAtReferenceContext(ctx, record.ReferenceID, record.TaskID, record.RequestID, TaskRequestSnapshotOriginal,
		"node-a", profile.ID, object.ObjectKey, int64(len(body)), fmt.Sprintf("%x", sha256.Sum256(body)), ExtractTaskRequestEvidence(body))
	require.ErrorIs(t, err, context.Canceled)
	_, err = GetStorageProfileByID(profile.ID, ctx)
	require.ErrorIs(t, err, context.Canceled)
	_, err = GetActiveStorageCredential(profile.ID, ctx)
	require.ErrorIs(t, err, context.Canceled)
	err = MarkStorageObjectUploaded(object.ID, "etag", "sha", ctx)
	require.ErrorIs(t, err, context.Canceled)
	err = BindTaskRequestSnapshots("cancel-task", "cancel-request", ctx)
	require.ErrorIs(t, err, context.Canceled)

	var unchanged TaskRequestBody
	require.NoError(t, db.First(&unchanged, record.ID).Error)
	assert.Equal(t, string(body), string(unchanged.Body))
	assert.Empty(t, unchanged.ObjectKey)
	require.NoError(t, db.First(&object, object.ID).Error)
	assert.Equal(t, StorageObjectStatusUploading, object.Status)
}

func TestLegacySnapshotMetadataPagesKeepPayloadsOutOfPrefetch(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}, &Log{}))
	previousLogDB := LOG_DB
	LOG_DB = db
	originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
	})
	task := Task{TaskID: "page-task", Properties: Properties{RequestBody: []byte(`{"from":"legacy-task"}`)}}
	require.NoError(t, db.Create(&task).Error)
	log := Log{RequestId: "page-request", Other: `{"request_body":{"from":"legacy-log"}}`}
	require.NoError(t, db.Create(&log).Error)
	tasks, err := FindLegacyTaskRequestBodyBatch(context.Background(), 0, task.ID, 1, true)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, task.TaskID, tasks[0].TaskID)
	assert.Empty(t, tasks[0].Properties.RequestBody)
	fullTasks, err := FindLegacyTaskRequestBodyBatch(context.Background(), 0, task.ID, 1)
	require.NoError(t, err)
	require.Len(t, fullTasks, 1)
	assert.Equal(t, string(task.Properties.RequestBody), string(fullTasks[0].Properties.RequestBody))
	upper, err := GetLogRequestBodyArchiveUpperCursor(context.Background())
	require.NoError(t, err)
	logs, err := FindLogRequestBodyArchiveBatch(context.Background(), LogRequestBodyArchiveCursor{}, upper, 1, true)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, log.RequestId, logs[0].RequestID)
	assert.Empty(t, logs[0].Other)
	fullLogs, err := FindLogRequestBodyArchiveBatch(context.Background(), LogRequestBodyArchiveCursor{}, upper, 1)
	require.NoError(t, err)
	require.Len(t, fullLogs, 1)
	assert.Equal(t, log.Other, fullLogs[0].Other)
}

func TestSnapshotLookupUsesCompatibleSmallLinkageQueriesAcrossDatabases(t *testing.T) {
	previousDB := DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Cleanup(func() {
		DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	for _, test := range []struct {
		name       string
		kind       common.DatabaseType
		dialector  gorm.Dialector
		projection string
	}{
		{"sqlite", common.DatabaseTypeSQLite, sqlite.Open(":memory:"), "json_extract(properties, '$.request_id')"},
		{"mysql", common.DatabaseTypeMySQL, mysql.New(mysql.Config{DSN: "gorm:gorm@tcp(127.0.0.1:3306)/gorm", SkipInitializeWithVersion: true}), "JSON_UNQUOTE(JSON_EXTRACT(properties, '$.request_id'))"},
		{"postgres", common.DatabaseTypePostgreSQL, postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=gorm dbname=gorm", PreferSimpleProtocol: true}), "properties->>'request_id'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &snapshotLookupSQLRecorder{}
			db, err := gorm.Open(test.dialector, &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: recorder})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			DB = db
			common.SetDatabaseTypes(test.kind, previousLogType)
			_, err = GetSnapshotRecords("missing-task", "")
			require.NoError(t, err)
			require.Len(t, recorder.statements, 5)
			assert.Contains(t, recorder.statements[4], test.projection)
			for _, statement := range recorder.statements {
				statement = strings.ToLower(strings.NewReplacer("`", "", "\"", "").Replace(statement))
				projection := strings.SplitN(statement, " from ", 2)[0]
				assert.NotContains(t, projection, "select *", "lookup sorting cannot include a large JSON body on any database")
				assert.NotContains(t, projection, ".body,", "body payloads belong only in the subsequent primary-key read")
				assert.NotContains(t, statement, " or task_id =", "reference and task lookups must retain separate indexable plans")
			}
		})
	}
}
