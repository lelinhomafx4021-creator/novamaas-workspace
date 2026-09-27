package model

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPollHistoryPreservesChangedResponsesAndDeduplicatesIdenticalOnes(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	taskID := "video-history"
	require.NoError(t, SaveTaskRequestSnapshots(taskID, "request-history", []byte(`{"prompt":"hello"}`), []byte(`{"model":"video"}`)))

	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"progress":10,"status":"running"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"status":"running", "progress":10}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"progress":20,"status":"running"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 429, "rate limited", []byte(`{"error":"rate limited"}`)))

	entries, err := ListTaskPollHistory(ctx, taskID, 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, 429, entries[0].HTTPStatus)
	assert.Equal(t, 2, entries[2].RepeatCount)
	assert.JSONEq(t, `{"progress":20,"status":"running"}`, string(entries[1].Response))

	firstPage, err := ListTaskPollHistory(ctx, taskID, 0, 1)
	require.NoError(t, err)
	secondPage, err := ListTaskPollHistory(ctx, taskID, firstPage[0].ID, 1)
	require.NoError(t, err)
	assert.Equal(t, entries[1].ID, secondPage[0].ID)

	snapshots, exists, err := GetArchivedRequestSnapshots(taskID, "request-history")
	require.NoError(t, err)
	require.True(t, exists)
	assert.JSONEq(t, `{"prompt":"hello"}`, string(snapshots.Original))
	assert.JSONEq(t, `{"model":"video"}`, string(snapshots.Upstream))
	var count int64
	require.NoError(t, db.Model(&TaskRequestBody{}).Count(&count).Error)
	assert.EqualValues(t, 5, count)
}

func TestTaskPollHistoryPreservesAdminResponseAndOmitsLargeResponses(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	require.NoError(t, RecordTaskPollHistory(ctx, "admin-task", "IN_PROGRESS", 200, "upstream token=abc", []byte(`{"api_key":"private","usage":{"total_tokens":123},"url":"https://example.com/video?token=secret&part=1","media":"data:image/png;base64,AAAA"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, "admin-task", "IN_PROGRESS", 200, "", bytes.Repeat([]byte("x"), MaxTaskPollResponseBytes+1)))
	entries, err := ListTaskPollHistory(ctx, "admin-task", 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.True(t, entries[0].ResponseOmitted)
	assert.Empty(t, entries[0].Response)
	assert.Equal(t, MaxTaskPollResponseBytes+1, entries[0].ResponseSize)
	assert.Equal(t, "upstream token=abc", entries[1].Error)
	assert.Contains(t, string(entries[1].Response), `"api_key":"private"`)
	assert.Contains(t, string(entries[1].Response), "token=secret")
	assert.True(t, entries[1].ResponseTruncated)
	assert.NotContains(t, string(entries[1].Response), "data:image/png;base64,AAAA")
	assert.Contains(t, string(entries[1].Response), "[content omitted:")
	assert.Contains(t, string(entries[1].Response), `"total_tokens":123`)
}

func TestTaskPollHistoryOmitsBase64ButKeepsChangingPollMetadata(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	const taskID = "base64-video"
	makeResponse := func(progress int, media string) []byte {
		response, err := common.Marshal(map[string]any{
			"status": "running", "progress": progress,
			"result": map[string]any{"video": "data:video/mp4;base64," + media, "b64_json": "QUJD"},
		})
		require.NoError(t, err)
		return response
	}
	first := makeResponse(20, strings.Repeat("A", MaxTaskPollResponseBytes))
	second := makeResponse(40, strings.Repeat("B", MaxTaskPollResponseBytes))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", first))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", first))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", second))

	entries, err := ListTaskPollHistory(ctx, taskID, 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, 2, entries[1].RepeatCount)
	assert.True(t, entries[0].ResponseTruncated)
	assert.False(t, entries[0].ResponseOmitted)
	assert.Less(t, len(entries[0].Response), MaxTaskPollResponseBytes)
	assert.JSONEq(t, `{"status":"running","progress":40,"result":{"video":"[content omitted: 65558 bytes]","b64_json":"[content omitted: 4 bytes]"}}`, string(entries[0].Response))
	assert.NotEqual(t, entries[0].ResponseSHA256, entries[1].ResponseSHA256)
}

func TestTaskPollHistoryCleanupRemovesOnlyExpiredPollRows(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	taskID := "cleanup-video"
	require.NoError(t, SaveTaskRequestBody(taskID, "", []byte(`{"prompt":"retain"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "QUEUED", 200, "", []byte(`{"status":"queued"}`)))
	old := time.Now().AddDate(0, 0, -91).Unix()
	require.NoError(t, db.Model(&TaskRequestBody{}).Where("reference_id LIKE ?", taskPollHistoryPrefix+"%").Update("created_at", old).Error)
	require.NoError(t, db.Model(&TaskRequestBody{}).Where("reference_id LIKE ?", taskPollHistoryPrefix+"%").Update("updated_at", old).Error)

	deleted, err := DeleteExpiredTaskPollHistory(ctx, time.Now(), 500)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	entries, err := ListTaskPollHistory(ctx, taskID, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
	body, exists, err := GetTaskRequestBody(taskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.JSONEq(t, `{"prompt":"retain"}`, string(body))

	// A new poll row remains within the retention window.
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "SUCCESS", 200, "", []byte(`{"status":"success"}`)))
	deleted, err = DeleteExpiredTaskPollHistory(ctx, time.Now(), 500)
	require.NoError(t, err)
	assert.Zero(t, deleted)
}

func TestTaskPollHistoryCleanupKeepsRecentlyRepeatedRows(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	const taskID = "repeated-video"
	response := []byte(`{"status":"running","progress":10}`)
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", response))
	old := time.Now().AddDate(0, 0, -91).Unix()
	require.NoError(t, db.Model(&TaskRequestBody{}).Where("reference_id LIKE ?", taskPollHistoryPrefix+"%").Updates(map[string]any{
		"created_at": old,
		"updated_at": old,
	}).Error)

	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", response))
	deleted, err := DeleteExpiredTaskPollHistory(ctx, time.Now(), 500)
	require.NoError(t, err)
	assert.Zero(t, deleted)
	entries, err := ListTaskPollHistory(ctx, taskID, 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 2, entries[0].RepeatCount)
}
