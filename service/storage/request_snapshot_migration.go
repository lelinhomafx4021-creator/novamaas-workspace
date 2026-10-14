package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// This runs inside the administrator's background migration job. Pagination
// reads only small metadata and each payload is loaded and released separately.
func MigrateDatabaseRequestSnapshots(ctx context.Context, afterID int64, batchSize int, clearBody bool) (cursor int64, processed int, done bool, err error) {
	return (snapshotObjectStore{}).migrateDatabaseRequestSnapshots(ctx, afterID, batchSize, clearBody)
}

func (store snapshotObjectStore) migrateDatabaseRequestSnapshots(ctx context.Context, afterID int64, batchSize int, clearBody bool) (cursor int64, processed int, done bool, err error) {
	if _, _, err := resolveRequestSnapshotPolicy(ctx); err != nil {
		return afterID, 0, false, err
	}
	return store.archiveDatabaseRequestSnapshots(ctx, afterID, batchSize, clearBody, true)
}

// Exact-primary-key operations let the maintenance scheduler prefetch small
// metadata and run a bounded number of independent payloads concurrently.
func MigrateDatabaseRequestSnapshot(ctx context.Context, id int64, clearBody bool) (bool, error) {
	return (snapshotObjectStore{}).archiveDatabaseRequestSnapshot(ctx, id, clearBody, true)
}

func CleanupDatabaseRequestSnapshot(ctx context.Context, id int64, clearBody bool) (bool, error) {
	return (snapshotObjectStore{}).archiveDatabaseRequestSnapshot(ctx, id, clearBody, false)
}

// Cleanup never uploads or changes a locator. Only an existing ready object
// whose exact bytes match the current inline body may replace that inline copy.
func CleanupDatabaseRequestSnapshots(ctx context.Context, afterID int64, batchSize int, clearBody bool) (cursor int64, processed int, done bool, err error) {
	return (snapshotObjectStore{}).archiveDatabaseRequestSnapshots(ctx, afterID, batchSize, clearBody, false)
}

func (store snapshotObjectStore) archiveDatabaseRequestSnapshots(ctx context.Context, afterID int64, batchSize int, clearBody, upload bool) (cursor int64, processed int, done bool, err error) {
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 20
	}
	rows, err := model.FindDatabaseSnapshotBatch(afterID, batchSize, ctx)
	if err != nil {
		return afterID, 0, false, err
	}
	cursor = afterID
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return cursor, processed, false, err
		}
		if !upload && (row.StorageState != model.TaskRequestSnapshotReady || row.ObjectKey == "") {
			cursor = row.ID
			continue
		}
		archived, err := store.archiveDatabaseRequestSnapshot(ctx, row.ID, clearBody, upload)
		if err != nil {
			return cursor, processed, false, err
		}
		cursor = row.ID
		if archived {
			processed++
		}
	}
	return cursor, processed, len(rows) < batchSize, nil
}

func (store snapshotObjectStore) archiveDatabaseRequestSnapshot(ctx context.Context, id int64, clearBody, upload bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	record, err := model.GetDatabaseSnapshotBody(id, ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(record.Body) == 0 || !upload && (record.StorageState != model.TaskRequestSnapshotReady || record.ObjectKey == "") {
		return false, nil
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(record.Body))
	if record.StorageState == model.TaskRequestSnapshotReady && record.ObjectKey != "" {
		if digest != record.BodySHA256 || int64(len(record.Body)) != record.BodySize {
			return false, errors.New("inline snapshot differs from its published object; inline copy preserved")
		}
		driver := store.driver
		if driver == nil {
			_, driver, err = openSnapshotDriver(ctx, record.StorageProfileID)
			if err != nil {
				return false, err
			}
		}
		if err := verifySnapshotObject(ctx, driver, record.ObjectKey, record.BodySize, digest); err != nil {
			return false, err
		}
	} else {
		policy, profile, err := resolveRequestSnapshotPolicy(ctx)
		if err != nil {
			return false, err
		}
		if int64(len(record.Body)) > policy.MaxFileBytes {
			return false, errors.New("legacy snapshot exceeds the configured file limit")
		}
		createdAt := record.RequestCreatedAt
		if createdAt <= 0 {
			createdAt = record.CreatedAt
		}
		capturedAt := record.CapturedAt
		if capturedAt <= 0 {
			capturedAt = createdAt * int64(time.Second)
		}
		job := &snapshotSpoolJob{
			SnapshotCapture: SnapshotCapture{ReferenceID: record.ReferenceID, TaskID: record.TaskID, RequestID: record.RequestID,
				Kind: record.Kind, NodeName: common.NodeName, Evidence: model.ExtractTaskRequestEvidence(record.Body), CapturedAt: capturedAt},
			Profile: *profile, PolicyID: policy.ID, Prefix: policy.ObjectPrefix, RetentionSeconds: policy.RetentionSeconds,
			Size: int64(len(record.Body)), SHA256: digest, CreatedAt: createdAt,
		}
		if err := store.put(ctx, job, bytes.NewReader(record.Body)); err != nil {
			return false, err
		}
	}
	if clearBody {
		cleared, err := model.ClearMigratedSnapshotBody(record.ID, digest, ctx)
		if err != nil {
			return false, err
		}
		if !cleared {
			_, currentErr := model.GetDatabaseSnapshotBody(record.ID, ctx)
			if errors.Is(currentErr, gorm.ErrRecordNotFound) {
				return false, nil
			}
			if currentErr != nil {
				return false, currentErr
			}
			return false, errors.New("snapshot changed before verified cleanup; inline copy preserved")
		}
	}
	return true, nil
}
