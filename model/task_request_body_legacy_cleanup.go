package model

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// ClearLegacyTaskSnapshotBody removes a hot-row body only after its exact bytes
// have a ready original object index. The storage caller verifies the object
// before this call; the transaction protects current properties and the index.
func ClearLegacyTaskSnapshotBody(ctx context.Context, rowID int64, expectedSHA string) (bool, error) {
	cleared := false
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task struct {
			ID         int64
			TaskID     string
			Properties json.RawMessage `gorm:"type:json"`
		}
		err := lockForUpdate(tx).Model(&Task{}).Select("id", "task_id", "properties").Where("id = ?", rowID).First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(task.Properties) == 0 {
			return nil
		}
		var properties map[string]json.RawMessage
		if err := common.Unmarshal(task.Properties, &properties); err != nil {
			return err
		}
		body := properties["request_body"]
		if len(body) == 0 || task.TaskID == "" || fmt.Sprintf("%x", sha256.Sum256(body)) != expectedSHA {
			return nil
		}
		ready, err := readyOriginalSnapshotMatches(tx, task.TaskID, "", int64(len(body)), expectedSHA)
		if err != nil || !ready {
			return err
		}
		delete(properties, "request_body")
		encodedProperties, err := common.Marshal(properties)
		if err != nil {
			return err
		}
		result := tx.Model(&Task{}).Where("id = ?", task.ID).Updates(map[string]any{
			"properties": encodedProperties, "request_body_available": true,
		})
		cleared = result.RowsAffected == 1
		return result.Error
	})
	return cleared, err
}

// ExtractLegacyLogSnapshotBody preserves JSON-valued bodies without converting
// numbers or changing formatting. JSON held inside a string retains its decoded
// bytes. Both the uploader and cleanup use this same checksum.
func ExtractLegacyLogSnapshotBody(otherJSON string) (body []byte, taskID string, found bool, err error) {
	var other map[string]json.RawMessage
	if err := common.UnmarshalJsonStr(otherJSON, &other); err != nil {
		return nil, "", false, err
	}
	if rawTaskID, exists := other["task_id"]; exists {
		_ = common.Unmarshal(rawTaskID, &taskID)
	}
	raw, found := other["request_body"]
	if !found {
		return nil, taskID, false, nil
	}
	if common.GetJsonType(raw) == "string" {
		var text string
		if err := common.Unmarshal(raw, &text); err != nil {
			return nil, taskID, true, err
		}
		if gjson.Valid(text) {
			return []byte(text), taskID, true, nil
		}
		body, err = common.Marshal(text)
		return body, taskID, true, err
	}
	if !gjson.ValidBytes(raw) {
		return nil, taskID, true, errors.New("invalid legacy log request body JSON")
	}
	return raw, taskID, true, nil
}

// ClearLegacyLogSnapshotBody accepts the scanned Other or its whole-document
// SHA-256 so a migration can release large scanned payloads before this read.
// It preserves every unrelated JSON field and refuses a changed hot log row.
func ClearLegacyLogSnapshotBody(ctx context.Context, row LogRequestBodyArchiveRow, taskID, expectedSHA string) (bool, error) {
	otherSHA := row.OtherSHA256
	if row.Other != "" {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(row.Other)))
		if otherSHA != "" && otherSHA != digest {
			return false, nil
		}
		otherSHA = digest
	}
	if len(otherSHA) != sha256.Size*2 {
		return false, errors.New("legacy log snapshot source checksum is required")
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		var current LogRequestBodyArchiveRow
		err := LOG_DB.WithContext(ctx).Model(&Log{}).Select("id, created_at, request_id, other").
			Where("created_at = ? AND request_id = ? AND lower(hex(SHA256(other))) = ?", row.CreatedAt, row.RequestID, otherSHA).
			Limit(1).Scan(&current).Error
		if err != nil || current.Other == "" {
			return false, err
		}
		newOther, ready, err := legacyLogSnapshotCleanupPayload(DB.WithContext(ctx), current, taskID, expectedSHA)
		if err != nil || !ready {
			return false, err
		}
		err = UpdateArchivedLogRequestBodies(ctx, []LogRequestBodyArchiveUpdate{{
			ID: current.ID, CreatedAt: current.CreatedAt, RequestID: current.RequestID,
			OriginalOther: current.Other, Other: newOther,
		}})
		return err == nil, err
	}
	cleared := false
	err := LOG_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current Log
		err := lockForUpdate(tx, true).Where("id = ?", row.ID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if current.CreatedAt != row.CreatedAt || current.RequestId != row.RequestID || fmt.Sprintf("%x", sha256.Sum256([]byte(current.Other))) != otherSHA {
			return nil
		}
		currentRow := LogRequestBodyArchiveRow{ID: current.Id, CreatedAt: current.CreatedAt, RequestID: current.RequestId, Other: current.Other}
		archiveScope := DB.WithContext(ctx)
		if LOG_DB == DB {
			archiveScope = tx
		}
		newOther, ready, err := legacyLogSnapshotCleanupPayload(archiveScope, currentRow, taskID, expectedSHA)
		if err != nil || !ready {
			return err
		}
		result := tx.Model(&Log{}).Where("id = ? AND other = ?", current.Id, current.Other).Update("other", newOther)
		cleared = result.RowsAffected == 1
		return result.Error
	})
	return cleared, err
}

func readyOriginalSnapshotMatches(scope *gorm.DB, taskID, requestID string, size int64, digest string) (bool, error) {
	if taskID == "" && requestID == "" {
		return false, nil
	}
	referenceID, err := taskRequestBodyReference(taskID, "")
	if requestID != "" {
		referenceID, err = taskRequestBodyReference("", requestID)
	}
	if err != nil {
		return false, err
	}
	query := scope.Model(&TaskRequestBody{}).
		Where("reference_id = ?", referenceID).
		Where("storage_state = ? AND storage_profile_id > 0 AND object_key <> ? AND body_size = ? AND body_sha256 = ?", TaskRequestSnapshotReady, "", size, digest).
		Where("kind = ? OR (kind = ? AND reference_id NOT LIKE ?)", TaskRequestSnapshotOriginal, "", upstreamRequestBodyReferencePrefix+"%").
		Where("reference_id NOT LIKE ?", taskPollHistoryPrefix+"%")
	if taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if requestID != "" {
		query = query.Where("request_id = ?", requestID)
	}
	var record TaskRequestBody
	err = lockForUpdate(query).Select("id").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func legacyLogSnapshotCleanupPayload(archiveScope *gorm.DB, row LogRequestBodyArchiveRow, taskID, expectedSHA string) (string, bool, error) {
	body, sourceTaskID, found, err := ExtractLegacyLogSnapshotBody(row.Other)
	if err != nil || !found {
		return "", false, err
	}
	if sourceTaskID != taskID || fmt.Sprintf("%x", sha256.Sum256(body)) != expectedSHA {
		return "", false, nil
	}
	ready, err := readyOriginalSnapshotMatches(archiveScope, taskID, row.RequestID, int64(len(body)), expectedSHA)
	if err != nil || !ready {
		return "", false, err
	}
	var other map[string]json.RawMessage
	if err := common.UnmarshalJsonStr(row.Other, &other); err != nil {
		return "", false, err
	}
	delete(other, "request_body")
	other["is_task"], other["request_body_available"] = json.RawMessage("true"), json.RawMessage("true")
	if taskID != "" {
		other["task_id"], err = common.Marshal(taskID)
	} else {
		other["request_body_ref"], err = common.Marshal("request:" + row.RequestID)
	}
	if err != nil {
		return "", false, err
	}
	encoded, err := common.Marshal(other)
	return string(encoded), err == nil, err
}
