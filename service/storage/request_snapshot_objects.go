package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Verified reads stream to their destination and always inspect one extra byte
// so neither a truncated nor an oversized remote object can pass verification.
func copyVerifiedSnapshot(reader io.Reader, writer io.Writer, size int64, digest string) error {
	if size <= 0 || size > 512*1024*1024 {
		return errors.New("invalid snapshot object size")
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(writer, hash), io.LimitReader(reader, size+1))
	if err != nil {
		return err
	}
	if written != size || hex.EncodeToString(hash.Sum(nil)) != digest {
		return errors.New("request snapshot integrity check failed")
	}
	return nil
}

func verifySnapshotObject(ctx context.Context, driver ObjectDriver, key string, size int64, digest string) error {
	body, err := driver.GetObject(ctx, key)
	if err != nil {
		return err
	}
	defer body.Close()
	return copyVerifiedSnapshot(body, io.Discard, size, digest)
}

func snapshotObjectKey(job *snapshotSpoolJob) string {
	referenceHash := sha256.Sum256([]byte(job.reference()))
	return fmt.Sprintf("%s/profile-%d/%x/%s/%s.json", strings.Trim(job.Prefix, "/"), job.Profile.ID, referenceHash, job.Kind, job.SHA256)
}

func registerSnapshotObject(ctx context.Context, job *snapshotSpoolJob, key string) (*model.StorageObject, error) {
	objectHash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", job.Profile.ID, key)))
	object := &model.StorageObject{
		ObjectID: hex.EncodeToString(objectHash[:]), OwnerUserID: job.UserID, RequestID: job.RequestID,
		TaskID: job.TaskID, StorageProfileID: job.Profile.ID, StoragePolicyID: job.PolicyID,
		Purpose: model.StorageObjectPurposeRequestSnapshot, ObjectKey: key, ContentType: "application/json",
		Size: job.Size, SHA256: job.SHA256, Status: model.StorageObjectStatusUploading,
	}
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		profile, err := model.LockStorageProfile(tx, job.Profile.ID)
		if err != nil {
			return err
		}
		if profile.Status == model.StorageProfileStatusArchived || profile.ProviderType != job.Profile.ProviderType || profile.Endpoint != job.Profile.Endpoint || profile.Region != job.Profile.Region || profile.Bucket != job.Profile.Bucket {
			return errors.New("snapshot storage identity changed before upload")
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(object).Error
	})
	if err != nil {
		return nil, err
	}
	object.ID = 0
	if err := model.DB.WithContext(ctx).Where("object_id = ?", object.ObjectID).First(object).Error; err != nil {
		return nil, err
	}
	if object.StorageProfileID != job.Profile.ID || object.ObjectKey != key || object.Size != job.Size || object.SHA256 != job.SHA256 || object.Purpose != model.StorageObjectPurposeRequestSnapshot {
		return nil, errors.New("request snapshot storage ledger conflicts with immutable object metadata")
	}
	if object.Status == model.StorageObjectStatusDeleted || object.Status == model.StorageObjectStatusDeleting || object.Status == model.StorageObjectStatusDeletePending {
		return nil, errors.New("request snapshot object has expired or is being deleted")
	}
	return object, nil
}

func putSnapshotObject(ctx context.Context, job *snapshotSpoolJob, reader io.ReadSeeker) error {
	return (snapshotObjectStore{}).put(ctx, job, reader)
}

type snapshotObjectStore struct {
	driver     ObjectDriver
	openDriver func(context.Context, int) (*model.StorageProfile, ObjectDriver, error)
}

func (store snapshotObjectStore) put(ctx context.Context, job *snapshotSpoolJob, reader io.ReadSeeker) error {
	key := snapshotObjectKey(job)
	object, err := registerSnapshotObject(ctx, job, key)
	if err != nil {
		return err
	}
	// Registering first freezes bucket/endpoint under the profile edit lock.
	// Opening a driver before registration permits an ABA profile edit to bind
	// the uploaded bytes to a different bucket than the published locator.
	driver := store.driver
	if driver == nil {
		openDriver := store.openDriver
		if openDriver == nil {
			openDriver = openSnapshotDriver
		}
		_, driver, err = openDriver(ctx, job.Profile.ID)
		if err != nil {
			return err
		}
	}
	if err := copyVerifiedSnapshot(reader, io.Discard, job.Size, job.SHA256); err != nil {
		return err
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if verifySnapshotObject(ctx, driver, key, job.Size, job.SHA256) != nil {
		etag, putErr := driver.PutObject(ctx, key, "application/json", job.Size, reader)
		// A timed-out upload or an immutable retry may have already succeeded.
		// Read back exact bytes before publishing any database locator.
		if err := verifySnapshotObject(ctx, driver, key, job.Size, job.SHA256); err != nil {
			if putErr != nil {
				return fmt.Errorf("upload request snapshot: %w", putErr)
			}
			return err
		}
		if err := model.MarkStorageObjectUploaded(object.ID, etag, job.SHA256, ctx); err != nil {
			return err
		}
	} else if err := model.MarkStorageObjectUploaded(object.ID, object.ETag, job.SHA256, ctx); err != nil {
		return err
	}
	if err := model.SaveExternalTaskRequestSnapshotAtReferenceContext(ctx, job.ReferenceID, job.TaskID, job.RequestID, job.Kind, job.NodeName, job.Profile.ID, key, job.Size, job.SHA256, job.Evidence, job.CapturedAt); err != nil {
		return err
	}
	if job.RetentionSeconds > 0 {
		return model.DB.WithContext(ctx).Model(&model.StorageObject{}).
			Where("id = ? AND delete_after = 0", object.ID).
			Update("delete_after", common.GetTimestamp()+job.RetentionSeconds).Error
	}
	return nil
}

func uploadSnapshotJob(ctx context.Context, spoolDir string, job *snapshotSpoolJob) error {
	path := filepath.Join(spoolDir, job.ID+".body")
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != job.Size {
		return errors.New("request snapshot spool body is missing or changed")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return putSnapshotObject(ctx, job, file)
}

func ReadSnapshotObject(ctx context.Context, record model.TaskRequestBody) ([]byte, error) {
	if record.StorageState != model.TaskRequestSnapshotReady || record.StorageProfileID <= 0 || record.ObjectKey == "" {
		return nil, errors.New("request snapshot object is not ready")
	}
	_, driver, err := openSnapshotDriver(ctx, record.StorageProfileID)
	if err != nil {
		return nil, err
	}
	body, err := driver.GetObject(ctx, record.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var payload bytes.Buffer
	if err := copyVerifiedSnapshot(body, &payload, record.BodySize, record.BodySHA256); err != nil {
		return nil, err
	}
	if !gjson.ValidBytes(payload.Bytes()) {
		return nil, errors.New("request snapshot object is not valid JSON")
	}
	return payload.Bytes(), nil
}

func ReadArchivedRequestSnapshots(ctx context.Context, taskID, requestID string) (model.TaskRequestSnapshots, bool, error) {
	records, err := model.GetSnapshotRecordsContext(ctx, taskID, requestID)
	if err != nil {
		return model.TaskRequestSnapshots{}, false, err
	}
	result := model.TaskRequestSnapshots{}
	for _, record := range records {
		body, err := readArchivedSnapshotRecord(ctx, record)
		if err != nil {
			return result, false, err
		}
		if len(body) == 0 {
			continue
		}
		if record.Kind == model.TaskRequestSnapshotUpstream {
			result.Upstream = body
		} else {
			result.Original = body
		}
	}
	return result, len(result.Original) > 0 || len(result.Upstream) > 0, nil
}

func ReadArchivedRequestBody(ctx context.Context, taskID, requestID string) (json.RawMessage, bool, error) {
	return ReadArchivedRequestSnapshot(ctx, taskID, requestID, model.TaskRequestSnapshotOriginal)
}

func ReadArchivedRequestSnapshot(ctx context.Context, taskID, requestID, kind string) (json.RawMessage, bool, error) {
	if kind != model.TaskRequestSnapshotOriginal && kind != model.TaskRequestSnapshotUpstream {
		return nil, false, errors.New("invalid request snapshot kind")
	}
	records, err := model.GetSnapshotRecordsContext(ctx, taskID, requestID)
	if err != nil {
		return nil, false, err
	}
	for _, record := range records {
		if record.Kind != kind {
			continue
		}
		body, err := readArchivedSnapshotRecord(ctx, record)
		return body, len(body) > 0, err
	}
	return nil, false, nil
}

func readArchivedSnapshotRecord(ctx context.Context, record model.TaskRequestBody) ([]byte, error) {
	body := []byte(record.Body)
	if record.StorageState == model.TaskRequestSnapshotReady && record.ObjectKey != "" {
		var err error
		body, err = ReadSnapshotObject(ctx, record)
		if err != nil && ctx.Err() == nil {
			inline := record.Body
			if len(inline) == 0 && record.ID > 0 {
				// Ready records normally carry only small locator metadata. The
				// retained migration copy is loaded only when the object read fails.
				legacy, legacyErr := model.GetDatabaseSnapshotBody(record.ID, ctx)
				if legacyErr != nil && !errors.Is(legacyErr, gorm.ErrRecordNotFound) {
					return nil, legacyErr
				}
				inline = legacy.Body
			}
			if len(inline) > 0 && copyVerifiedSnapshot(bytes.NewReader(inline), io.Discard, record.BodySize, record.BodySHA256) == nil {
				body, err = inline, nil
			}
		}
		if err != nil {
			return nil, err
		}
	}
	if len(body) > 0 && !gjson.ValidBytes(body) {
		return nil, errors.New("archived request snapshot is not valid JSON")
	}
	return body, nil
}
