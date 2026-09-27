package model

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const taskPollHistoryPrefix = "poll:v1:"
const MaxTaskPollResponseBytes = 64 << 10
const maxTaskPollParseBytes = 8 << 20
const maxTaskPollStringBytes = 4 << 10
const taskPollHistoryDays = 90

// TaskPollHistoryEntry is stored in TaskRequestBody.Body under a separate
// reference prefix, leaving the two submission request snapshots untouched.
type TaskPollHistoryEntry struct {
	ID                int64           `json:"id"`
	TaskID            string          `json:"task_id"`
	FirstSeenAt       int64           `json:"first_seen_at"`
	LastSeenAt        int64           `json:"last_seen_at"`
	RepeatCount       int             `json:"repeat_count"`
	Status            string          `json:"status,omitempty"`
	HTTPStatus        int             `json:"http_status,omitempty"`
	Error             string          `json:"error,omitempty"`
	Response          json.RawMessage `json:"response,omitempty"`
	ResponseSize      int             `json:"response_size,omitempty"`
	ResponseSHA256    string          `json:"response_sha256,omitempty"`
	ResponseOmitted   bool            `json:"response_omitted,omitempty"`
	ResponseTruncated bool            `json:"response_truncated,omitempty"`
}

func omitTaskPollLargeStrings(value any, fieldName string) (any, bool) {
	switch current := value.(type) {
	case map[string]any:
		changed := false
		for key, item := range current {
			trimmed, itemChanged := omitTaskPollLargeStrings(item, key)
			current[key] = trimmed
			changed = changed || itemChanged
		}
		return current, changed
	case []any:
		changed := false
		for index, item := range current {
			trimmed, itemChanged := omitTaskPollLargeStrings(item, fieldName)
			current[index] = trimmed
			changed = changed || itemChanged
		}
		return current, changed
	case string:
		prefix := current
		if len(prefix) > 128 {
			prefix = prefix[:128]
		}
		lowerPrefix := strings.ToLower(prefix)
		isBase64URI := strings.HasPrefix(lowerPrefix, "data:") && strings.Contains(lowerPrefix, ";base64,")
		isBase64Field := strings.Contains(strings.ToLower(fieldName), "base64") || strings.EqualFold(fieldName, "b64_json")
		if len(current) > maxTaskPollStringBytes || isBase64URI || isBase64Field {
			return fmt.Sprintf("[content omitted: %d bytes]", len(current)), true
		}
	}
	return value, false
}

func taskPollResponse(raw []byte) (json.RawMessage, string, bool, bool) {
	if len(raw) == 0 {
		return nil, "", false, false
	}
	if len(raw) > maxTaskPollParseBytes {
		digest := sha256.Sum256(raw)
		return nil, fmt.Sprintf("%x", digest), true, false
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		if len(raw) > MaxTaskPollResponseBytes {
			digest := sha256.Sum256(raw)
			return nil, fmt.Sprintf("%x", digest), true, false
		}
		value = string(raw)
	}
	var digest [32]byte
	if len(raw) > MaxTaskPollResponseBytes {
		digest = sha256.Sum256(raw)
	} else {
		canonical, err := common.Marshal(value)
		if err != nil {
			return nil, fmt.Sprintf("%x", sha256.Sum256(raw)), true, false
		}
		digest = sha256.Sum256(canonical)
	}
	value, truncated := omitTaskPollLargeStrings(value, "")
	clean, err := common.Marshal(value)
	if err != nil || len(clean) > MaxTaskPollResponseBytes {
		return nil, fmt.Sprintf("%x", digest), true, truncated
	}
	return clean, fmt.Sprintf("%x", digest), false, truncated
}

func RecordTaskPollHistory(ctx context.Context, taskID, status string, httpStatus int, pollError string, raw []byte) error {
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	response, responseHash, omitted, truncated := taskPollResponse(raw)
	if len(pollError) > 2048 {
		pollError = pollError[:2048]
	}
	now := time.Now().Unix()
	entry := TaskPollHistoryEntry{
		TaskID: taskID, FirstSeenAt: now, LastSeenAt: now, RepeatCount: 1,
		Status: status, HTTPStatus: httpStatus, Error: pollError,
		Response: response, ResponseSize: len(raw), ResponseSHA256: responseHash,
		ResponseOmitted: omitted, ResponseTruncated: truncated,
	}
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the task row even when this is its first poll history entry.
		// The latest archive row does not exist yet in that case.
		var task Task
		if err := lockForUpdate(tx).Select("id").Where("task_id = ?", taskID).First(&task).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var last TaskRequestBody
		err := lockForUpdate(tx).Where("task_id = ? AND reference_id LIKE ?", taskID, taskPollHistoryPrefix+"%").Order("id DESC").First(&last).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			var previous TaskPollHistoryEntry
			if err := common.Unmarshal(last.Body, &previous); err != nil {
				return err
			}
			if previous.Status == entry.Status && previous.HTTPStatus == entry.HTTPStatus &&
				previous.Error == entry.Error && previous.ResponseSHA256 == entry.ResponseSHA256 &&
				previous.ResponseOmitted == entry.ResponseOmitted && previous.ResponseTruncated == entry.ResponseTruncated {
				previous.LastSeenAt = now
				previous.RepeatCount++
				body, err := common.Marshal(previous)
				if err != nil {
					return err
				}
				digest := sha256.Sum256(body)
				return tx.Model(&TaskRequestBody{}).Where("id = ?", last.ID).Updates(map[string]any{
					"body": json.RawMessage(body), "body_size": len(body),
					"body_sha256": fmt.Sprintf("%x", digest), "updated_at": now,
				}).Error
			}
		}
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return err
		}
		taskDigest := sha256.Sum256([]byte(taskID))
		body, err := common.Marshal(entry)
		if err != nil {
			return err
		}
		bodyDigest := sha256.Sum256(body)
		record := TaskRequestBody{
			ReferenceID: taskPollHistoryPrefix + fmt.Sprintf("%x:%x", taskDigest[:16], random),
			TaskID:      taskID, Body: body, BodySize: int64(len(body)),
			BodySHA256: fmt.Sprintf("%x", bodyDigest), RequestCreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&record).Error
	})
}

func ListTaskPollHistory(ctx context.Context, taskID string, beforeID int64, limit int) ([]TaskPollHistoryEntry, error) {
	if limit < 1 || limit > 101 {
		limit = 50
	}
	query := DB.WithContext(ctx).Select("id", "body").Where("task_id = ? AND reference_id LIKE ?", taskID, taskPollHistoryPrefix+"%")
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}
	var rows []TaskRequestBody
	if err := query.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	entries := make([]TaskPollHistoryEntry, 0, len(rows))
	for _, row := range rows {
		var entry TaskPollHistoryEntry
		if err := common.Unmarshal(row.Body, &entry); err != nil {
			return nil, err
		}
		entry.ID = row.ID
		entries = append(entries, entry)
	}
	return entries, nil
}

func DeleteExpiredTaskPollHistory(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit < 1 || limit > 1000 {
		limit = 500
	}
	cutoff := now.AddDate(0, 0, -taskPollHistoryDays).Unix()
	var ids []int64
	// A deduplicated row can keep receiving identical polls after its first
	// observation. Retain it until its last observation is also 90 days old.
	if err := DB.WithContext(ctx).Model(&TaskRequestBody{}).Where("reference_id LIKE ? AND created_at < ? AND updated_at < ?", taskPollHistoryPrefix+"%", cutoff, cutoff).
		Order("id ASC").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.WithContext(ctx).Where("id IN ? AND reference_id LIKE ? AND created_at < ? AND updated_at < ?", ids, taskPollHistoryPrefix+"%", cutoff, cutoff).Delete(&TaskRequestBody{})
	return result.RowsAffected, result.Error
}
