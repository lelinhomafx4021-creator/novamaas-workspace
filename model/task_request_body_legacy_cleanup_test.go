package model

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLegacySnapshotCleanupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}, &Log{}))
	previousLogDB := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previousLogDB })
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func saveLegacySnapshotIndex(t *testing.T, taskID, requestID string, body []byte) string {
	t.Helper()
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	require.NoError(t, SaveExternalTaskRequestSnapshot(taskID, requestID, TaskRequestSnapshotOriginal,
		"legacy-node", 7, "snapshots/"+digest+".json", int64(len(body)), digest, ExtractTaskRequestEvidence(body)))
	return digest
}

func TestLegacyTaskSnapshotCleanupPreservesCurrentTaskProperties(t *testing.T) {
	db := setupLegacySnapshotCleanupDB(t)
	task := Task{TaskID: "legacy-properties", Status: TaskStatusSuccess, Quota: 23,
		Properties: Properties{Input: "keep input", RequestID: "keep-request", OriginModelName: "public-model", UpstreamModelName: "provider-model",
			SnapshotKinds: []string{TaskRequestSnapshotOriginal}, RequestBody: json.RawMessage(`{"content":[{"type":"text"}]}`),
			RequestEvidence: &TaskRequestEvidence{Version: 1, Valid: true, Resolution: "720p"}}}
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, db.First(&task, task.ID).Error)
	expectedProperties := task.Properties
	digest := saveLegacySnapshotIndex(t, task.TaskID, "", task.Properties.RequestBody)
	storedProperties, err := task.Properties.Value()
	require.NoError(t, err)
	var currentProperties map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(storedProperties.([]byte), &currentProperties))
	currentProperties["future_metric"] = json.RawMessage(`9007199254740993`)
	currentProperties["future_options"] = json.RawMessage(`{"keep":"new schema field"}`)
	encodedProperties, err := common.Marshal(currentProperties)
	require.NoError(t, err)
	require.NoError(t, db.Model(&Task{}).Where("id = ?", task.ID).Update("properties", encodedProperties).Error)

	cleared, err := ClearLegacyTaskSnapshotBody(context.Background(), task.ID, digest)
	require.NoError(t, err)
	require.True(t, cleared)
	var stored Task
	require.NoError(t, db.First(&stored, task.ID).Error)
	expectedProperties.RequestBody = nil
	assert.Equal(t, expectedProperties, stored.Properties)
	assert.True(t, stored.RequestBodyAvailable)
	assert.Equal(t, task.Status, stored.Status)
	assert.Equal(t, 23, stored.Quota)
	var rawProperties struct {
		Properties json.RawMessage `gorm:"type:json"`
	}
	require.NoError(t, db.Model(&Task{}).Select("properties").Where("id = ?", task.ID).Scan(&rawProperties).Error)
	var remainingProperties map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(rawProperties.Properties, &remainingProperties))
	assert.Equal(t, "9007199254740993", string(remainingProperties["future_metric"]))
	assert.JSONEq(t, `{"keep":"new schema field"}`, string(remainingProperties["future_options"]))
	var archive TaskRequestBody
	require.NoError(t, db.First(&archive).Error)
	assert.Empty(t, archive.Body, "hot-table migration must not create another database body copy")
}

func TestLegacyTaskSnapshotCleanupRejectsChangedBodyAndIncompleteArchives(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Task, *TaskRequestBody)
	}{
		{"changed_body", func(task *Task, _ *TaskRequestBody) {
			task.Properties.RequestBody = json.RawMessage(`{"prompt":"new request"}`)
		}},
		{"pending_archive", func(_ *Task, archive *TaskRequestBody) { archive.StorageState = TaskRequestSnapshotPending }},
		{"failed_archive", func(_ *Task, archive *TaskRequestBody) { archive.StorageState = TaskRequestSnapshotFailed }},
		{"missing_locator", func(_ *Task, archive *TaskRequestBody) { archive.ObjectKey = "" }},
		{"wrong_size", func(_ *Task, archive *TaskRequestBody) { archive.BodySize++ }},
		{"wrong_task", func(_ *Task, archive *TaskRequestBody) { archive.TaskID = "other-task" }},
		{"upstream_only", func(_ *Task, archive *TaskRequestBody) { archive.Kind = TaskRequestSnapshotUpstream }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupLegacySnapshotCleanupDB(t)
			task := Task{TaskID: "guarded-task", Properties: Properties{Input: "keep", RequestBody: json.RawMessage(`{"prompt":"original"}`)}}
			require.NoError(t, db.Create(&task).Error)
			require.NoError(t, db.First(&task, task.ID).Error)
			digest := saveLegacySnapshotIndex(t, task.TaskID, "", task.Properties.RequestBody)
			var archive TaskRequestBody
			require.NoError(t, db.First(&archive).Error)
			tc.change(&task, &archive)
			require.NoError(t, db.Save(&task).Error)
			require.NoError(t, db.Save(&archive).Error)
			cleared, err := ClearLegacyTaskSnapshotBody(context.Background(), task.ID, digest)
			require.NoError(t, err)
			assert.False(t, cleared)
			var stored Task
			require.NoError(t, db.First(&stored, task.ID).Error)
			assert.Equal(t, string(task.Properties.RequestBody), string(stored.Properties.RequestBody))
			assert.False(t, stored.RequestBodyAvailable)
		})
	}
}

func TestLegacyTaskAndLogSnapshotsPreserveDifferentOriginalFormatting(t *testing.T) {
	db := setupLegacySnapshotCleanupDB(t)
	formattedBody := `{ "z": 1, "a": 2 }`
	task := Task{TaskID: "canonical-task", Properties: Properties{RequestBody: json.RawMessage(formattedBody)}}
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, db.First(&task, task.ID).Error)
	canonicalBody := append([]byte(nil), task.Properties.RequestBody...)
	require.NotEqual(t, formattedBody, string(canonicalBody), "JSON-valued task properties compact their nested JSON")
	taskDigest := saveLegacySnapshotIndex(t, task.TaskID, "", canonicalBody)
	encodedBody, err := common.Marshal(formattedBody)
	require.NoError(t, err)
	other := `{"task_id":"canonical-task","billing_total":9007199254740993,"marker":{"note":"keep"},"request_body":` + string(encodedBody) + `}`
	log := Log{CreatedAt: 100, RequestId: "formatted-log-request", Other: other}
	require.NoError(t, db.Create(&log).Error)
	row := LogRequestBodyArchiveRow{ID: log.Id, CreatedAt: log.CreatedAt, RequestID: log.RequestId, Other: log.Other}
	logDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(formattedBody)))

	cleared, err := ClearLegacyLogSnapshotBody(context.Background(), row, task.TaskID, logDigest)
	require.NoError(t, err)
	assert.False(t, cleared, "a differently formatted task archive cannot authorize log cleanup")
	saveLegacySnapshotIndex(t, "", log.RequestId, []byte(formattedBody))
	require.NoError(t, BindTaskRequestSnapshots(task.TaskID, log.RequestId))

	// A compact source checksum allows the caller to release the original large
	// Other string before cleanup reloads it under the log transaction.
	row.OtherSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(row.Other)))
	row.Other = ""
	cleared, err = ClearLegacyLogSnapshotBody(context.Background(), row, task.TaskID, logDigest)
	require.NoError(t, err)
	require.True(t, cleared)
	cleared, err = ClearLegacyTaskSnapshotBody(context.Background(), task.ID, taskDigest)
	require.NoError(t, err)
	require.True(t, cleared)
	var storedLog Log
	require.NoError(t, db.First(&storedLog, log.Id).Error)
	var fields map[string]json.RawMessage
	require.NoError(t, common.UnmarshalJsonStr(storedLog.Other, &fields))
	assert.NotContains(t, fields, "request_body")
	assert.Equal(t, "9007199254740993", string(fields["billing_total"]))
	assert.JSONEq(t, `{"note":"keep"}`, string(fields["marker"]))
	assert.Equal(t, "true", string(fields["is_task"]))
	assert.Equal(t, "true", string(fields["request_body_available"]))
	assert.Equal(t, `"canonical-task"`, string(fields["task_id"]))

	taskRecords, err := GetSnapshotRecords(task.TaskID, "", true)
	require.NoError(t, err)
	require.Len(t, taskRecords, 1)
	assert.Equal(t, taskDigest, taskRecords[0].BodySHA256)
	logRecords, err := GetSnapshotRecords(task.TaskID, log.RequestId, true)
	require.NoError(t, err)
	require.Len(t, logRecords, 1)
	assert.Equal(t, logDigest, logRecords[0].BodySHA256)
	assert.NotEqual(t, taskRecords[0].ObjectKey, logRecords[0].ObjectKey)
}

func TestLegacyLogSnapshotCleanupRejectsChangedOtherAndPendingArchive(t *testing.T) {
	for _, tc := range []struct {
		name, currentOther string
		state              string
	}{
		{"changed_unrelated_field", `{"marker":"new","request_body":"{\"prompt\":\"original\"}"}`, TaskRequestSnapshotReady},
		{"changed_request_body", `{"marker":"old","request_body":"{\"prompt\":\"changed\"}"}`, TaskRequestSnapshotReady},
		{"pending_archive", `{"marker":"old","request_body":"{\"prompt\":\"original\"}"}`, TaskRequestSnapshotPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupLegacySnapshotCleanupDB(t)
			body := []byte(`{"prompt":"original"}`)
			originalOther := `{"marker":"old","request_body":"{\"prompt\":\"original\"}"}`
			log := Log{CreatedAt: 100, RequestId: "guarded-log-request", Other: originalOther}
			require.NoError(t, db.Create(&log).Error)
			row := LogRequestBodyArchiveRow{ID: log.Id, CreatedAt: log.CreatedAt, RequestID: log.RequestId, Other: log.Other}
			digest := saveLegacySnapshotIndex(t, "", log.RequestId, body)
			require.NoError(t, db.Model(&TaskRequestBody{}).Where("request_id = ?", log.RequestId).Update("storage_state", tc.state).Error)
			require.NoError(t, db.Model(&Log{}).Where("id = ?", log.Id).Update("other", tc.currentOther).Error)
			cleared, err := ClearLegacyLogSnapshotBody(context.Background(), row, "", digest)
			require.NoError(t, err)
			assert.False(t, cleared)
			var stored Log
			require.NoError(t, db.First(&stored, log.Id).Error)
			assert.Equal(t, tc.currentOther, stored.Other)
		})
	}
}

func TestExtractLegacyLogSnapshotBodyPreservesOriginalJSON(t *testing.T) {
	for _, tc := range []struct{ name, other, body string }{
		{"json_string_preserves_whitespace", `{"task_id":"task","request_body":"{ \"a\": 1 }"}`, `{ "a": 1 }`},
		{"structured_body_preserves_key_order", `{"request_body":{"z":1,"a":2}}`, `{"z":1,"a":2}`},
		{"structured_body_preserves_large_integer", `{"request_body":{"id":9007199254740993}}`, `{"id":9007199254740993}`},
		{"structured_body_preserves_whitespace", `{"request_body":{ "z" : 1, "a": 2 }}`, `{ "z" : 1, "a": 2 }`},
		{"explicit_null_body", `{"request_body":null}`, `null`},
		{"plain_string", `{"request_body":"plain text"}`, `"plain text"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _, found, err := ExtractLegacyLogSnapshotBody(tc.other)
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, tc.body, string(body))
		})
	}
}
