package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const supplierTestPurpose = "supplier_test"

// PutSupplierVideoEvidence reuses configured policies, private JSON storage,
// object ownership and the ordinary retention cleanup ledger.
func PutSupplierVideoEvidence(ctx context.Context, ownerID int, runID string, body []byte) (*model.StorageObject, error) {
	if ownerID <= 0 || !strings.HasPrefix(runID, "svr_") {
		return nil, errors.New("invalid supplier test object identity")
	}
	policy, profile, credential, err := ResolveEnabledPolicy(model.StoragePolicyRequestSnapshot)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || int64(len(body)) > policy.MaxFileBytes {
		return nil, errors.New("test object exceeds the configured storage limit")
	}
	driver, err := driverForProfile(profile, credential)
	if err != nil {
		return nil, err
	}
	suffix, err := randomObjectSuffix()
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(body)
	object := &model.StorageObject{ObjectID: "obj_" + suffix, OwnerUserID: ownerID, StorageProfileID: profile.ID, StoragePolicyID: policy.ID, Purpose: supplierTestPurpose, ObjectKey: fmt.Sprintf("%s/supplier-test/user-%s/%s/%s-%s", strings.Trim(policy.ObjectPrefix, "/"), permanentStorageUserHash(ownerID), runID, "evidence", suffix), ContentType: "application/json", Size: int64(len(body)), SHA256: hex.EncodeToString(digest[:]), Status: model.StorageObjectStatusUploading}
	if policy.RetentionSeconds > 0 {
		object.DeleteAfter = time.Now().Unix() + policy.RetentionSeconds
	}
	if err = model.CreateStorageObject(object); err != nil {
		return nil, err
	}
	etag, err := driver.PutObject(ctx, object.ObjectKey, object.ContentType, object.Size, bytes.NewReader(body))
	if err != nil {
		_ = model.MarkStorageObjectForDeletion(object.ID, time.Now().Unix(), "supplier test upload failed")
		return nil, err
	}
	if err = model.MarkStorageObjectUploaded(object.ID, etag, object.SHA256); err != nil {
		return nil, err
	}
	object.Status = model.StorageObjectStatusUploaded
	return object, nil
}

func ReadSupplierVideoEvidence(ctx context.Context, objectID string, ownerID int) ([]byte, error) {
	object, driver, err := openSupplierTestObject(ctx, objectID, ownerID)
	if err != nil {
		return nil, err
	}
	if object.Size <= 0 || object.Size > 128<<20 {
		return nil, errors.New("invalid evidence size")
	}
	reader, err := driver.GetObject(ctx, object.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var body bytes.Buffer
	if err = copyVerifiedSnapshot(reader, &body, object.Size, object.SHA256); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func openSupplierTestObject(ctx context.Context, objectID string, ownerID int) (*model.StorageObject, ObjectDriver, error) {
	var object model.StorageObject
	if err := model.DB.WithContext(ctx).Where("object_id = ? AND owner_user_id = ? AND purpose = ? AND status = ?", objectID, ownerID, supplierTestPurpose, model.StorageObjectStatusUploaded).First(&object).Error; err != nil {
		return nil, nil, err
	}
	if object.DeleteAfter > 0 && object.DeleteAfter <= time.Now().Unix() {
		return nil, nil, errors.New("test evidence has expired")
	}
	profile, err := model.GetStorageProfileByID(object.StorageProfileID)
	if err != nil {
		return nil, nil, err
	}
	credential, err := model.GetActiveStorageCredential(object.StorageProfileID)
	if err != nil {
		return nil, nil, err
	}
	driver, err := driverForProfile(profile, credential)
	return &object, driver, err
}
