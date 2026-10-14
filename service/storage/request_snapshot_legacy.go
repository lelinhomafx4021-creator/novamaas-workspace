package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/tidwall/gjson"
)

func MigrateLegacyTaskRequestSnapshots(ctx context.Context, afterID, upperID int64, batchSize int, clearBody bool) (int64, int, bool, error) {
	return archiveLegacyTaskRequestSnapshots(ctx, afterID, upperID, batchSize, clearBody, true)
}

func CleanupLegacyTaskRequestSnapshots(ctx context.Context, afterID, upperID int64, batchSize int, clearBody bool) (int64, int, bool, error) {
	return archiveLegacyTaskRequestSnapshots(ctx, afterID, upperID, batchSize, clearBody, false)
}

func archiveLegacyTaskRequestSnapshots(ctx context.Context, cursor, upperID int64, batchSize int, clearBody, upload bool) (int64, int, bool, error) {
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 20
	}
	processed := 0
	for range batchSize {
		if err := ctx.Err(); err != nil {
			return cursor, processed, false, err
		}
		rows, err := model.FindLegacyTaskRequestBodyBatch(ctx, cursor, upperID, 1)
		if err != nil {
			return cursor, processed, false, err
		}
		if len(rows) == 0 {
			return cursor, processed, true, nil
		}
		row := rows[0]
		if len(row.Properties.RequestBody) == 0 {
			cursor = row.ID
			continue
		}
		createdAt := row.SubmitTime
		if createdAt <= 0 {
			createdAt = row.CreatedAt
		}
		digest, archived, err := (snapshotObjectStore{}).archiveLegacyRequestSnapshot(ctx, row.TaskID, "", createdAt, row.Properties.RequestBody, upload)
		if err != nil {
			return cursor, processed, false, err
		}
		if !archived {
			cursor = row.ID
			continue
		}
		// Release the first large row before the model's CAS reloads current
		// properties under its row lock. No intermediate DB Body is written.
		row.Properties.RequestBody, rows[0].Properties.RequestBody = nil, nil
		if clearBody {
			cleared, err := model.ClearLegacyTaskSnapshotBody(ctx, row.ID, digest)
			if err != nil {
				return cursor, processed, false, err
			}
			if !cleared {
				cursor = row.ID
				continue
			}
		}
		processed++
		cursor = row.ID
	}
	return cursor, processed, cursor >= upperID, nil
}

func MigrateLegacyLogRequestSnapshots(ctx context.Context, cursor, upper model.LogRequestBodyArchiveCursor, batchSize int, clearBody bool) (model.LogRequestBodyArchiveCursor, int, bool, error) {
	return archiveLegacyLogRequestSnapshots(ctx, cursor, upper, batchSize, clearBody, true)
}

func CleanupLegacyLogRequestSnapshots(ctx context.Context, cursor, upper model.LogRequestBodyArchiveCursor, batchSize int, clearBody bool) (model.LogRequestBodyArchiveCursor, int, bool, error) {
	return archiveLegacyLogRequestSnapshots(ctx, cursor, upper, batchSize, clearBody, false)
}

func archiveLegacyLogRequestSnapshots(ctx context.Context, cursor, upper model.LogRequestBodyArchiveCursor, batchSize int, clearBody, upload bool) (model.LogRequestBodyArchiveCursor, int, bool, error) {
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 20
	}
	processed := 0
	for range batchSize {
		if err := ctx.Err(); err != nil {
			return cursor, processed, false, err
		}
		rows, err := model.FindLogRequestBodyArchiveBatch(ctx, cursor, upper, 1)
		if err != nil {
			return cursor, processed, false, err
		}
		if len(rows) == 0 {
			return cursor, processed, true, nil
		}
		row := rows[0]
		next := model.LogRequestBodyArchiveCursor{ID: row.ID, CreatedAt: row.CreatedAt, RequestID: row.RequestID, OtherSHA256: row.OtherSHA256}
		body, taskID, exists, extractErr := model.ExtractLegacyLogSnapshotBody(row.Other)
		if extractErr != nil || !exists {
			cursor = next
			continue
		}
		if taskID == "" && row.RequestID == "" {
			cursor = next
			continue
		}
		sourceTaskID := taskID
		if row.RequestID != "" {
			// MySQL task JSON can have different bytes than the raw log TEXT.
			// A request reference preserves both sources without overwriting.
			sourceTaskID = ""
		}
		digest, archived, err := (snapshotObjectStore{}).archiveLegacyRequestSnapshot(ctx, sourceTaskID, row.RequestID, row.CreatedAt, body, upload)
		if err != nil {
			return cursor, processed, false, err
		}
		if !archived {
			cursor = next
			continue
		}
		if taskID != "" && row.RequestID != "" {
			if err := model.BindTaskRequestSnapshots(taskID, row.RequestID, ctx); err != nil {
				return cursor, processed, false, err
			}
		}
		if row.OtherSHA256 == "" {
			hash := sha256.New()
			_, err := io.Copy(hash, strings.NewReader(row.Other))
			if err != nil {
				return cursor, processed, false, err
			}
			row.OtherSHA256 = fmt.Sprintf("%x", hash.Sum(nil))
		}
		row.Other, rows[0].Other, body = "", "", nil
		if clearBody {
			cleared, err := model.ClearLegacyLogSnapshotBody(ctx, row, taskID, digest)
			if err != nil {
				return cursor, processed, false, err
			}
			if !cleared {
				cursor = next
				continue
			}
		}
		processed++
		cursor = next
	}
	return cursor, processed, false, nil
}

// This migrates directly from a hot-table payload to an immutable object. A
// different ready snapshot is never replaced or used to authorize cleanup.
func (store snapshotObjectStore) archiveLegacyRequestSnapshot(ctx context.Context, taskID, requestID string, createdAt int64, body []byte, upload bool) (string, bool, error) {
	if !gjson.ValidBytes(body) {
		return "", false, errors.New("legacy request snapshot is not valid JSON")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	records, err := model.GetSnapshotRecordsContext(ctx, taskID, requestID, true)
	if err != nil {
		return digest, false, err
	}
	for _, record := range records {
		if taskID != "" && requestID == "" && record.ReferenceID != taskID {
			continue
		}
		if record.Kind != model.TaskRequestSnapshotOriginal || record.StorageState != model.TaskRequestSnapshotReady || record.ObjectKey == "" {
			continue
		}
		if record.BodySHA256 != digest || record.BodySize != int64(len(body)) {
			return digest, false, errors.New("legacy request differs from its published snapshot; original hot-table copy preserved")
		}
		driver := store.driver
		if driver == nil {
			_, driver, err = openSnapshotDriver(ctx, record.StorageProfileID)
			if err != nil {
				return digest, false, err
			}
		}
		err = verifySnapshotObject(ctx, driver, record.ObjectKey, record.BodySize, digest)
		return digest, err == nil, err
	}
	if !upload {
		return digest, false, nil
	}
	policy, profile, err := resolveRequestSnapshotPolicy(ctx)
	if err != nil {
		return digest, false, err
	}
	if int64(len(body)) > policy.MaxFileBytes {
		return digest, false, errors.New("legacy request snapshot exceeds the configured file limit")
	}
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}
	job := &snapshotSpoolJob{
		SnapshotCapture: SnapshotCapture{TaskID: taskID, RequestID: requestID, Kind: model.TaskRequestSnapshotOriginal,
			NodeName: common.NodeName, Evidence: model.ExtractTaskRequestEvidence(body), CapturedAt: createdAt * int64(time.Second)},
		Profile: *profile, PolicyID: policy.ID, Prefix: policy.ObjectPrefix, RetentionSeconds: policy.RetentionSeconds,
		Size: int64(len(body)), SHA256: digest, CreatedAt: createdAt,
	}
	if err := store.put(ctx, job, bytes.NewReader(body)); err != nil {
		return digest, false, err
	}
	// A newer capture can win the publication CAS while this upload is running.
	// Only the locator actually published for these exact bytes permits cleanup.
	records, err = model.GetSnapshotRecordsContext(ctx, taskID, requestID, true)
	if err != nil {
		return digest, false, err
	}
	for _, record := range records {
		if taskID != "" && requestID == "" && record.ReferenceID != taskID {
			continue
		}
		if record.Kind == model.TaskRequestSnapshotOriginal && record.StorageState == model.TaskRequestSnapshotReady && record.BodySHA256 == digest && record.BodySize == int64(len(body)) && record.ObjectKey == snapshotObjectKey(job) {
			return digest, true, nil
		}
	}
	return digest, false, errors.New("legacy snapshot publication changed during migration; original copy preserved")
}
