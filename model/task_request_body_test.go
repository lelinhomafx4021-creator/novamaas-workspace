package model

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTaskRequestBodyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TaskRequestBody{}))
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	return db
}

func TestSaveTaskRequestSnapshotsStoresBodiesInSeparateRows(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	original := []byte(`{"model":"public-model","prompt":"hello"}`)
	upstream := []byte(`{"model":"upstream-model","content":[{"type":"text","text":"hello"}]}`)

	require.NoError(t, SaveTaskRequestSnapshots("task_snapshot", "request_snapshot", original, upstream))

	snapshots, exists, err := GetArchivedRequestSnapshots("task_snapshot", "request_snapshot")
	require.NoError(t, err)
	require.True(t, exists)
	assert.JSONEq(t, string(original), string(snapshots.Original))
	assert.JSONEq(t, string(upstream), string(snapshots.Upstream))

	var records []TaskRequestBody
	require.NoError(t, db.Order("reference_id ASC").Find(&records).Error)
	require.Len(t, records, 2)
	assert.NotEqual(t, records[0].ReferenceID, records[1].ReferenceID)
	for _, record := range records {
		assert.LessOrEqual(t, record.BodySize, int64(MaxTaskRequestBodyBytes))
		assert.Len(t, record.BodySHA256, 64)
	}
}

func TestSaveTaskRequestSnapshotsRejectsOversizedUpstreamBodyAtomically(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	original := []byte(`{"prompt":"safe"}`)
	upstream := bytes.Repeat([]byte(" "), MaxTaskRequestBodyBytes+1)

	err := SaveTaskRequestSnapshots("task_oversized", "request_oversized", original, upstream)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTaskRequestBodyTooLarge))
	var count int64
	require.NoError(t, db.Model(&TaskRequestBody{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestUpstreamRequestBodyReferenceIsBoundedForMaximumTaskID(t *testing.T) {
	reference, err := upstreamRequestBodyReference(string(bytes.Repeat([]byte("t"), 191)), "")

	require.NoError(t, err)
	assert.LessOrEqual(t, len(reference), 191)
	assert.Len(t, reference, len(upstreamRequestBodyReferencePrefix)+sha256.Size*2)
}

func TestExtractTaskRequestEvidencePreservesBillingInputs(t *testing.T) {
	for _, tc := range []struct {
		name, body, resolution string
		valid, video           bool
	}{
		{"native", `{"resolution":" 1080P ","content":[{"type":"text"}]}`, "1080p", true, false},
		{"generic_metadata", `{"metadata":{"resolution":"720p","content":[{"type":"video_url"}]}}`, "720p", true, true},
		{"top_level_precedence", `{"content":[{"type":"text"}],"metadata":{"content":[{"type":"video_url"}]}}`, "", true, false},
		{"video_field", `{"content":[{"type":"image_url","video_url":{"url":"https://example.test/video.mp4"}}]}`, "", true, true},
		{"null_video_field", `{"content":[{"type":"image_url","video_url":null}]}`, "", true, true},
		{"empty_video_object", `{"content":[{"type":"image_url","video_url":{}}]}`, "", true, true},
		{"invalid_content", `{"content":[{"type":"unknown"}]}`, "", false, false},
		{"numeric_resolution", `{"resolution":720,"content":[{"type":"text"}]}`, "", false, false},
		{"empty_content", `{"content":[]}`, "", false, false},
		{"invalid_json", `{`, "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evidence := ExtractTaskRequestEvidence([]byte(tc.body))
			assert.Equal(t, TaskRequestEvidenceVersion, evidence.Version)
			assert.Equal(t, tc.valid, evidence.Valid)
			assert.Equal(t, tc.resolution, evidence.Resolution)
			assert.Equal(t, tc.video, evidence.HasVideoInput)
		})
	}
}

func TestExtractTaskRequestEvidenceHandlesLargeVideoURLWithoutChangingPricingInputs(t *testing.T) {
	body := []byte(`{"resolution":"720p","content":[{"type":"image_url","video_url":{"url":"data:video/mp4;base64,` + strings.Repeat("A", 8*1024*1024) + `"}}]}`)
	evidence := ExtractTaskRequestEvidence(body)
	assert.Equal(t, TaskRequestEvidence{Version: TaskRequestEvidenceVersion, Valid: true, Resolution: "720p", HasVideoInput: true}, evidence)
}

func BenchmarkExtractTaskRequestEvidenceVideoURL(b *testing.B) {
	for _, size := range []int{32 * 1024, 8 * 1024 * 1024} {
		b.Run(fmt.Sprintf("%dKiB", size/1024), func(b *testing.B) {
			body := []byte(`{"resolution":"720p","content":[{"type":"video_url","video_url":{"url":"data:video/mp4;base64,` + strings.Repeat("A", size) + `"}}]}`)
			require.True(b, ExtractTaskRequestEvidence(body).HasVideoInput)
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ExtractTaskRequestEvidence(body)
			}
		})
	}
}

func TestSnapshotMigrationRetainsInlineBodyUntilVerifiedCleanup(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	body := []byte(`{"resolution":"720p","content":[{"type":"video_url"}]}`)
	require.NoError(t, SaveTaskRequestBody("migration-task", "migration-request", body))
	var record TaskRequestBody
	require.NoError(t, db.First(&record).Error)

	cleared, err := ClearMigratedSnapshotBody(record.ID, record.BodySHA256)
	require.NoError(t, err)
	assert.False(t, cleared, "an inline-only snapshot is its sole durable copy")

	require.NoError(t, SaveExternalTaskRequestSnapshot(record.TaskID, record.RequestID, TaskRequestSnapshotOriginal, "node-a", 7, "snapshots/original.json", int64(len(body)), record.BodySHA256, ExtractTaskRequestEvidence(body)))
	require.NoError(t, db.First(&record, record.ID).Error)
	assert.Equal(t, string(body), string(record.Body), "publishing the index must retain the legacy copy")
	assert.True(t, record.EvidenceValid)
	assert.True(t, record.HasVideoInput)

	cleared, err = ClearMigratedSnapshotBody(record.ID, fmt.Sprintf("%064d", 0))
	require.NoError(t, err)
	assert.False(t, cleared, "a stale verification result must not clear the body")

	cleared, err = ClearMigratedSnapshotBody(record.ID, record.BodySHA256)
	require.NoError(t, err)
	assert.True(t, cleared)
	var stored TaskRequestBody
	require.NoError(t, db.First(&stored, record.ID).Error)
	assert.Empty(t, stored.Body)
	assert.Equal(t, "snapshots/original.json", stored.ObjectKey)
	assert.Equal(t, "720p", stored.Resolution)
	assert.Equal(t, record.BodySHA256, stored.BodySHA256)
}

func TestSnapshotCleanupRejectsChangedBytesAndPollingHistory(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	body := []byte(`{"content":[{"type":"text"}]}`)
	digest := sha256.Sum256(body)
	sha := fmt.Sprintf("%x", digest)
	records := []TaskRequestBody{
		{ReferenceID: "changed-task", Body: []byte(`{"content":[{"type":"video_url"}]}`), BodySize: int64(len(body)), BodySHA256: sha, StorageState: TaskRequestSnapshotReady, StorageProfileID: 7, ObjectKey: "snapshots/changed.json"},
		{ReferenceID: taskPollHistoryPrefix + "history", Body: body, BodySize: int64(len(body)), BodySHA256: sha, StorageState: TaskRequestSnapshotReady, StorageProfileID: 7, ObjectKey: "snapshots/poll.json"},
		{ReferenceID: "missing-locator", Body: body, BodySize: int64(len(body)), BodySHA256: sha, StorageState: TaskRequestSnapshotReady},
	}
	require.NoError(t, db.Create(&records).Error)
	for _, record := range records {
		cleared, err := ClearMigratedSnapshotBody(record.ID, sha)
		require.NoError(t, err)
		assert.False(t, cleared, record.ReferenceID)
		var stored TaskRequestBody
		require.NoError(t, db.First(&stored, record.ID).Error)
		assert.Equal(t, string(record.Body), string(stored.Body))
	}
	batch, err := FindDatabaseSnapshotBatch(0, 100)
	require.NoError(t, err)
	require.Len(t, batch, 2)
	assert.Equal(t, "changed-task", batch[0].ReferenceID)
	assert.Equal(t, "missing-locator", batch[1].ReferenceID)
	batch, err = FindDatabaseSnapshotBatch(batch[0].ID, 1)
	require.NoError(t, err)
	require.Len(t, batch, 1)
	assert.Equal(t, "missing-locator", batch[0].ReferenceID)
}

func TestSnapshotLookupFallsBackToRequestAndPrefersTaskRecords(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	task := Task{TaskID: "lookup-task", Properties: Properties{RequestID: "lookup-request"}}
	require.NoError(t, db.Create(&task).Error)
	body := []byte(`{"content":[{"type":"text"}]}`)
	digest := sha256.Sum256(body)
	sha := fmt.Sprintf("%x", digest)
	evidence := ExtractTaskRequestEvidence(body)
	require.NoError(t, SaveExternalTaskRequestSnapshot("", "lookup-request", TaskRequestSnapshotOriginal, "node-a", 7, "snapshots/request.json", int64(len(body)), sha, evidence))

	records, err := GetSnapshotRecords("lookup-task", "")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "snapshots/request.json", records[0].ObjectKey)

	require.NoError(t, BindTaskRequestSnapshots("lookup-task", "lookup-request"))
	require.NoError(t, SaveExternalTaskRequestSnapshot("lookup-task", "lookup-request", TaskRequestSnapshotOriginal, "node-a", 7, "snapshots/rebound.json", int64(len(body)), sha, evidence))
	var count int64
	require.NoError(t, db.Model(&TaskRequestBody{}).Count(&count).Error)
	assert.EqualValues(t, 1, count, "binding followed by publishing must reuse the request reference")

	require.NoError(t, SaveTaskRequestBody("lookup-task", "lookup-request", []byte(`{"prompt":"task body"}`)))
	records, err = GetSnapshotRecords("lookup-task", "")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.JSONEq(t, `{"prompt":"task body"}`, string(records[0].Body))
	records, err = GetSnapshotRecords("lookup-task", "lookup-request")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "snapshots/rebound.json", records[0].ObjectKey, "an explicit request trace selects its own original bytes")
}

func TestExternalSnapshotPublicationRejectsOlderCapture(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	oldBody := []byte(`{"resolution":"720p","content":[{"type":"text"}]}`)
	newBody := []byte(`{"resolution":"1080p","content":[{"type":"video_url"}]}`)
	newDigest, oldDigest := sha256.Sum256(newBody), sha256.Sum256(oldBody)
	require.NoError(t, SaveExternalTaskRequestSnapshot("order-task", "", TaskRequestSnapshotUpstream, "node-a", 7, "snapshots/new.json", int64(len(newBody)), fmt.Sprintf("%x", newDigest), ExtractTaskRequestEvidence(newBody), 200))
	require.NoError(t, SaveExternalTaskRequestSnapshot("order-task", "", TaskRequestSnapshotUpstream, "node-a", 7, "snapshots/old.json", int64(len(oldBody)), fmt.Sprintf("%x", oldDigest), ExtractTaskRequestEvidence(oldBody), 100))
	var record TaskRequestBody
	require.NoError(t, db.First(&record).Error)
	assert.Equal(t, "snapshots/new.json", record.ObjectKey)
	assert.Equal(t, "1080p", record.Resolution)
	assert.True(t, record.HasVideoInput)
	assert.EqualValues(t, 200, record.CapturedAt)
}

func TestSnapshotLookupFindsOriginalForFailedSubmissionWithoutTaskRow(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	original := []byte(`{"resolution":"720p","content":[{"type":"text"}]}`)
	upstream := []byte(`{"resolution":"1080p","content":[{"type":"text"}]}`)
	originalDigest, upstreamDigest := sha256.Sum256(original), sha256.Sum256(upstream)
	require.NoError(t, SaveExternalTaskRequestSnapshot("", "failed-request", TaskRequestSnapshotOriginal, "node-a", 7, "snapshots/original.json", int64(len(original)), fmt.Sprintf("%x", originalDigest), ExtractTaskRequestEvidence(original)))
	require.NoError(t, SaveExternalTaskRequestSnapshot("failed-task", "failed-request", TaskRequestSnapshotUpstream, "node-a", 7, "snapshots/upstream.json", int64(len(upstream)), fmt.Sprintf("%x", upstreamDigest), ExtractTaskRequestEvidence(upstream)))

	records, err := GetSnapshotRecords("failed-task", "")
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, TaskRequestSnapshotOriginal, records[0].Kind)
	assert.Equal(t, "snapshots/original.json", records[0].ObjectKey)
	assert.Equal(t, TaskRequestSnapshotUpstream, records[1].Kind)
	assert.Equal(t, "snapshots/upstream.json", records[1].ObjectKey)
}

func TestExplicitSnapshotReferenceKeepsTaskAndRequestArchiveRowsIndependent(t *testing.T) {
	for _, requestFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("request_first_%t", requestFirst), func(t *testing.T) {
			db := setupTaskRequestBodyTestDB(t)
			const taskID, requestID = "reference-task", "reference-request"
			taskBody := []byte(`{"z":1,"a":2}`)
			requestBody := []byte(`{ "z": 1, "a": 2 }`)
			if requestFirst {
				require.NoError(t, SaveTaskRequestBody("", requestID, requestBody))
			}
			require.NoError(t, SaveTaskRequestBody(taskID, requestID, taskBody))
			if !requestFirst {
				require.NoError(t, SaveTaskRequestBody("", requestID, requestBody))
			}
			require.NoError(t, BindTaskRequestSnapshots(taskID, requestID))
			var rows []TaskRequestBody
			require.NoError(t, db.Order("id ASC").Find(&rows).Error)
			require.Len(t, rows, 2)
			for _, row := range rows {
				key := "snapshots/" + row.ReferenceID + ".json"
				require.NoError(t, SaveExternalTaskRequestSnapshotAtReference(row.ReferenceID, row.TaskID, row.RequestID,
					row.Kind, "migration-node", 7, key, row.BodySize, row.BodySHA256, ExtractTaskRequestEvidence(row.Body), row.CapturedAt))
				cleared, err := ClearMigratedSnapshotBody(row.ID, row.BodySHA256)
				require.NoError(t, err)
				require.True(t, cleared, "the scanned reference must receive its own ready locator")
				var stored TaskRequestBody
				require.NoError(t, db.First(&stored, row.ID).Error)
				assert.Equal(t, row.ReferenceID, stored.ReferenceID)
				assert.Equal(t, row.BodySHA256, stored.BodySHA256)
				assert.Equal(t, key, stored.ObjectKey)
				assert.Empty(t, stored.Body)
			}
			taskRecords, err := GetSnapshotRecords(taskID, "", true)
			require.NoError(t, err)
			require.Len(t, taskRecords, 1)
			assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256(taskBody)), taskRecords[0].BodySHA256)
			requestRecords, err := GetSnapshotRecords(taskID, requestID, true)
			require.NoError(t, err)
			require.Len(t, requestRecords, 1)
			assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256(requestBody)), requestRecords[0].BodySHA256)
		})
	}
}

func TestExplicitSnapshotReferenceRejectsUnrelatedSourceAndKind(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	body := []byte(`{"content":[{"type":"text"}]}`)
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	upstreamReference, err := upstreamRequestBodyReference("explicit-task", "explicit-request")
	require.NoError(t, err)
	for _, tc := range []struct{ name, reference, taskID, requestID, kind string }{
		{"unrelated_request", "request:other", "explicit-task", "explicit-request", TaskRequestSnapshotOriginal},
		{"unrelated_task", "other-task", "explicit-task", "explicit-request", TaskRequestSnapshotOriginal},
		{"upstream_as_original", upstreamReference, "explicit-task", "explicit-request", TaskRequestSnapshotOriginal},
		{"original_as_upstream", "explicit-task", "explicit-task", "explicit-request", TaskRequestSnapshotUpstream},
		{"poll_namespace", taskPollHistoryPrefix + "body", taskPollHistoryPrefix + "body", "", TaskRequestSnapshotOriginal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := SaveExternalTaskRequestSnapshotAtReference(tc.reference, tc.taskID, tc.requestID, tc.kind, "node-a", 7,
				"snapshots/test.json", int64(len(body)), digest, ExtractTaskRequestEvidence(body))
			assert.ErrorContains(t, err, "reference does not match")
		})
	}
	var count int64
	require.NoError(t, db.Model(&TaskRequestBody{}).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, SaveExternalTaskRequestSnapshotAtReference(upstreamReference, "explicit-task", "explicit-request", TaskRequestSnapshotUpstream,
		"node-a", 7, "snapshots/upstream.json", int64(len(body)), digest, ExtractTaskRequestEvidence(body)))
	var stored TaskRequestBody
	require.NoError(t, db.First(&stored).Error)
	assert.Equal(t, upstreamReference, stored.ReferenceID)
	assert.Equal(t, TaskRequestSnapshotUpstream, stored.Kind)
}
