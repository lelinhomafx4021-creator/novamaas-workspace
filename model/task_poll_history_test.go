package model

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPollHistoryIgnoresTimestampOnlyChanges(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	taskID := "video-history"
	require.NoError(t, SaveTaskRequestSnapshots(taskID, "request-history", []byte(`{"prompt":"hello"}`), []byte(`{"model":"video"}`)))

	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "SUBMITTED", 200, "", []byte(`{"status":"submitted"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "QUEUED", 200, "", []byte(`{"status":"queued"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"progress":10,"status":"running","updated_at":100}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"updated_at":200,"status":"running", "progress":10}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"progress":20,"status":"running"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 429, "rate limited", []byte(`{"error":"rate limited"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "SUCCESS", 200, "", []byte(`{"status":"success"}`)))

	entries, err := ListTaskPollHistory(ctx, taskID, 0, 10)
	require.NoError(t, err)
	require.Len(t, entries, 6)
	assert.Equal(t, "SUCCESS", entries[0].Status)
	assert.Equal(t, "IN_PROGRESS", entries[1].Status)
	assert.Equal(t, 429, entries[1].HTTPStatus)
	assert.Equal(t, "rate limited", entries[1].Error)
	assert.JSONEq(t, `{"error":"rate limited"}`, string(entries[1].Response))
	assert.JSONEq(t, `{"progress":20,"status":"running"}`, string(entries[2].Response))
	assert.Equal(t, 2, entries[3].RepeatCount)
	assert.JSONEq(t, `{"updated_at":200,"status":"running","progress":10}`, string(entries[3].Response))
	assert.Equal(t, "QUEUED", entries[4].Status)
	assert.Equal(t, "SUBMITTED", entries[5].Status)

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
	assert.EqualValues(t, 8, count)
}

func TestTaskPollHistoryPreservesAdminResponsesAcrossLifecyclePhases(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	require.NoError(t, RecordTaskPollHistory(ctx, "admin-task", "IN_PROGRESS", 200, "upstream token=abc", []byte(`{"api_key":"private","usage":{"total_tokens":123},"url":"https://example.com/video?token=secret&part=1","media":"data:image/png;base64,AAAA"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, "admin-task", "SUCCESS", 200, "", bytes.Repeat([]byte("x"), MaxTaskPollResponseBytes+1)))
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
	assert.Equal(t, 1, entries[0].RepeatCount)
	assert.Equal(t, 2, entries[1].RepeatCount)
	assert.True(t, entries[0].ResponseTruncated)
	assert.False(t, entries[0].ResponseOmitted)
	assert.Less(t, len(entries[0].Response), MaxTaskPollResponseBytes)
	assert.JSONEq(t, `{"status":"running","progress":40,"result":{"video":"[content omitted: 65558 bytes]","b64_json":"[content omitted: 4 bytes]"}}`, string(entries[0].Response))
}

func TestListTaskPollHistoryPaginatesLegacyRowsByStoredID(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	ctx := context.Background()
	taskID := "legacy-video-history"
	entries := []TaskPollHistoryEntry{
		{TaskID: taskID, Status: "SUBMITTED", FirstSeenAt: 10, LastSeenAt: 11, RepeatCount: 2, Response: []byte(`{"status":"submitted"}`)},
		{TaskID: taskID, Status: "QUEUED", FirstSeenAt: 12, LastSeenAt: 13, RepeatCount: 3, Response: []byte(`{"status":"queued"}`)},
		{TaskID: taskID, Status: "IN_PROGRESS", FirstSeenAt: 14, LastSeenAt: 15, RepeatCount: 4, Response: []byte(`{"progress":30,"updated_at":100}`)},
		{TaskID: taskID, Status: "IN_PROGRESS", FirstSeenAt: 16, LastSeenAt: 17, RepeatCount: 5, Response: []byte(`{"progress":30,"updated_at":200}`)},
	}
	for index, entry := range entries {
		body, err := common.Marshal(entry)
		require.NoError(t, err)
		require.NoError(t, db.Create(&TaskRequestBody{
			ReferenceID: fmt.Sprintf("%slegacy-%d", taskPollHistoryPrefix, index),
			TaskID:      taskID,
			Body:        body,
		}).Error)
	}

	history, err := ListTaskPollHistory(ctx, taskID, 0, 10)
	require.NoError(t, err)
	require.Len(t, history, 4)
	assert.Equal(t, "IN_PROGRESS", history[0].Status)
	assert.Equal(t, 5, history[0].RepeatCount)
	assert.EqualValues(t, 16, history[0].FirstSeenAt)
	assert.EqualValues(t, 17, history[0].LastSeenAt)
	assert.JSONEq(t, `{"progress":30,"updated_at":200}`, string(history[0].Response))
	assert.Equal(t, "IN_PROGRESS", history[1].Status)
	assert.Equal(t, 4, history[1].RepeatCount)
	assert.JSONEq(t, `{"progress":30,"updated_at":100}`, string(history[1].Response))
	assert.Equal(t, "QUEUED", history[2].Status)
	assert.Equal(t, "SUBMITTED", history[3].Status)

	page, err := ListTaskPollHistory(ctx, taskID, history[0].ID, 2)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, history[1].ID, page[0].ID)
	assert.Equal(t, history[2].ID, page[1].ID)
}

func TestListTaskPollHistoryBoundsReadAcrossFormerScanBatches(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	ctx := context.Background()
	const taskID = "many-video-polls"
	// A legacy row with an incompatible body must not affect newer pages.
	require.NoError(t, db.Create(&TaskRequestBody{
		ReferenceID: taskPollHistoryPrefix + "incompatible-oldest",
		TaskID:      taskID,
		Body:        []byte(`"unexpected"`),
	}).Error)

	const validRows = 102 // Crosses the former 100-row scan batch boundary.
	stored := make([]TaskRequestBody, 0, validRows)
	for index := range validRows {
		body, err := common.Marshal(TaskPollHistoryEntry{
			TaskID: taskID, Status: fmt.Sprintf("STEP_%03d", index), RepeatCount: 1,
		})
		require.NoError(t, err)
		stored = append(stored, TaskRequestBody{
			ReferenceID: fmt.Sprintf("%smany-%03d", taskPollHistoryPrefix, index),
			TaskID:      taskID,
			Body:        body,
		})
	}
	require.NoError(t, db.CreateInBatches(&stored, 50).Error)

	var beforeID int64
	var seenIDs []int64
	for pageIndex, expectedSize := range []int{50, 50, 2} {
		page, err := ListTaskPollHistory(ctx, taskID, beforeID, expectedSize)
		require.NoError(t, err, "page %d", pageIndex)
		require.Len(t, page, expectedSize)
		for _, entry := range page {
			seenIDs = append(seenIDs, entry.ID)
		}
		beforeID = page[len(page)-1].ID
	}
	require.Len(t, seenIDs, validRows)
	for index, id := range seenIDs {
		assert.Equal(t, stored[validRows-1-index].ID, id)
	}

	_, err := ListTaskPollHistory(ctx, taskID, beforeID, 50)
	require.Error(t, err)
}

func TestTaskPollHistoryPreservesStatusTransitionsBackToEarlierStatus(t *testing.T) {
	db := setupTaskRequestBodyTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}))
	ctx := context.Background()
	const taskID = "status-regression"
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"progress":20,"timestamp":100}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"progress":20,"timestamp":123}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "QUEUED", 200, "", []byte(`{"status":"queued"}`)))
	require.NoError(t, RecordTaskPollHistory(ctx, taskID, "IN_PROGRESS", 200, "", []byte(`{"progress":50}`)))

	history, err := ListTaskPollHistory(ctx, taskID, 0, 10)
	require.NoError(t, err)
	require.Len(t, history, 3)
	assert.Equal(t, "IN_PROGRESS", history[0].Status)
	assert.Equal(t, 1, history[0].RepeatCount)
	assert.Equal(t, "QUEUED", history[1].Status)
	assert.Equal(t, "IN_PROGRESS", history[2].Status)
	assert.Equal(t, 2, history[2].RepeatCount)
	assert.JSONEq(t, `{"progress":20,"timestamp":123}`, string(history[2].Response))
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
