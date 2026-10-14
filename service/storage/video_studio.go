package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

// Video studio references share one ledger row per user file. A row is
// temporary while DeleteAfter > 0 and belongs to the user's library once
// DeleteAfter is cleared; Seedance assets may point at the same row.

type VideoStudioReference struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Saved       bool   `json:"saved"`
	ExpiresAt   int64  `json:"expires_at"`
	CreatedAt   int64  `json:"created_at"`
}

type VideoStudioReferencePage struct {
	Items    []VideoStudioReference `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

func GetVideoStudioPolicy() (*model.StoragePolicy, error) {
	policy, err := model.GetStoragePolicyByKey(model.StoragePolicyVideoStudio)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return model.DefaultVideoStudioStoragePolicy(), nil
	}
	return policy, nil
}

func SaveVideoStudioPolicy(input PolicyInput) (*model.StoragePolicy, error) {
	policy := &model.StoragePolicy{
		Key:                 model.StoragePolicyVideoStudio,
		Name:                "Video studio reference storage",
		Purpose:             model.StorageObjectPurposeVideoStudio,
		StorageProfileID:    input.StorageProfileID,
		ObjectPrefix:        strings.Trim(strings.TrimSpace(input.ObjectPrefix), "/"),
		SignedURLTTLSeconds: input.SignedURLTTLSeconds,
		RetentionSeconds:    input.RetentionSeconds,
		MaxFileBytes:        input.MaxFileBytes,
		MaxTotalBytes:       input.MaxTotalBytes,
		MaxFiles:            input.MaxFiles,
		AllowedMIMETypes:    model.VideoStudioAllowedMIMETypes,
		Enabled:             input.Enabled,
	}
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	if err := model.SaveStoragePolicy(policy); err != nil {
		return nil, err
	}
	return policy, nil
}

// UploadVideoStudioReference stores an image as a temporary reference. The
// same bytes uploaded again by the same user reuse the live ledger row.
func UploadVideoStudioReference(ctx context.Context, ownerUserID int, fileName string, body io.Reader) (*VideoStudioReference, error) {
	if ownerUserID <= 0 {
		return nil, &RequestError{StatusCode: http.StatusUnauthorized, Err: errors.New("login required")}
	}
	policy, profile, credential, err := ResolveEnabledPolicy(model.StoragePolicyVideoStudio)
	if err != nil {
		return nil, &RequestError{StatusCode: http.StatusServiceUnavailable, Err: fmt.Errorf("video studio storage is unavailable: %w", err)}
	}
	payload, err := io.ReadAll(io.LimitReader(body, policy.MaxFileBytes+1))
	if err != nil {
		return nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("read reference image failed")}
	}
	if len(payload) == 0 || int64(len(payload)) > policy.MaxFileBytes {
		return nil, &RequestError{StatusCode: http.StatusRequestEntityTooLarge, Err: fmt.Errorf("reference image must be between 1 and %d bytes", policy.MaxFileBytes)}
	}
	contentType := detectMediaContentType(payload)
	extension, supported := mediaExtension(contentType)
	if !supported || !policyAllowsMIMEType(policy, contentType) {
		return nil, &RequestError{StatusCode: http.StatusUnsupportedMediaType, Err: errors.New("reference image must be JPEG, PNG, or WebP")}
	}
	digest := sha256.Sum256(payload)
	hash := hex.EncodeToString(digest[:])
	fileName = sanitizeReferenceFileName(fileName, extension)
	now := common.GetTimestamp()

	var existing model.StorageObject
	err = liveVideoStudioObjects(model.DB, ownerUserID, now).Where("sha256 = ?", hash).Order("id desc").First(&existing).Error
	if err == nil {
		return videoStudioReferenceView(&existing), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var usedFiles int64
	if err = liveVideoStudioObjects(model.DB, ownerUserID, now).Count(&usedFiles).Error; err != nil {
		return nil, err
	}
	if usedFiles >= int64(policy.MaxFiles) {
		return nil, &RequestError{StatusCode: http.StatusInsufficientStorage, Err: errors.New("video studio file limit is reached; delete saved references first")}
	}
	var usedBytes int64
	if err = liveVideoStudioObjects(model.DB, ownerUserID, now).Select("COALESCE(SUM(size), 0)").Scan(&usedBytes).Error; err != nil {
		return nil, err
	}
	if usedBytes+int64(len(payload)) > policy.MaxTotalBytes {
		return nil, &RequestError{StatusCode: http.StatusInsufficientStorage, Err: errors.New("video studio storage quota is exhausted; delete saved references first")}
	}

	suffix, err := randomObjectSuffix()
	if err != nil {
		return nil, err
	}
	object := &model.StorageObject{
		ObjectID:         "ref_" + suffix,
		OwnerUserID:      ownerUserID,
		StorageProfileID: profile.ID,
		StoragePolicyID:  policy.ID,
		Purpose:          model.StorageObjectPurposeVideoStudio,
		ObjectKey: fmt.Sprintf("%s/user-%s/%s/%s.%s",
			strings.Trim(policy.ObjectPrefix, "/"), permanentStorageUserHash(ownerUserID), time.Now().UTC().Format("2006/01"), suffix, extension),
		FileName:    fileName,
		ContentType: contentType,
		Size:        int64(len(payload)),
		SHA256:      hash,
		Status:      model.StorageObjectStatusUploading,
		DeleteAfter: now + policy.RetentionSeconds,
	}
	if err = model.CreateStorageObject(object); err != nil {
		return nil, err
	}
	driver, err := driverForProfile(profile, credential)
	if err != nil {
		_ = model.MarkStorageObjectForDeletion(object.ID, now, "initialize video studio storage failed")
		return nil, &RequestError{StatusCode: http.StatusServiceUnavailable, Err: err}
	}
	etag, err := driver.PutObject(ctx, object.ObjectKey, contentType, object.Size, bytes.NewReader(payload))
	if err != nil {
		_ = model.MarkStorageObjectForDeletion(object.ID, now, "video studio reference upload failed")
		return nil, &RequestError{StatusCode: http.StatusBadGateway, Err: fmt.Errorf("upload reference image: %w", err)}
	}
	if err = model.MarkStorageObjectUploaded(object.ID, etag, hash); err != nil {
		_ = model.MarkStorageObjectForDeletion(object.ID, now, "persist video studio upload state failed")
		return nil, err
	}
	object.Status = model.StorageObjectStatusUploaded
	return videoStudioReferenceView(object), nil
}

// ListVideoStudioReferences returns the user's library files; temporary
// uploads are only listed when explicitly requested.
func ListVideoStudioReferences(ownerUserID int, includeTemporary bool, page int, pageSize int) (*VideoStudioReferencePage, error) {
	page = max(page, 1)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 40
	}
	query := liveVideoStudioObjects(model.DB, ownerUserID, common.GetTimestamp())
	if !includeTemporary {
		query = query.Where("delete_after = 0")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var objects []model.StorageObject
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&objects).Error; err != nil {
		return nil, err
	}
	items := make([]VideoStudioReference, 0, len(objects))
	for index := range objects {
		items = append(items, *videoStudioReferenceView(&objects[index]))
	}
	return &VideoStudioReferencePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// FindVideoStudioReference loads a live reference owned by the user. Expired
// temporary files are reported as not found even before physical cleanup.
func FindVideoStudioReference(ownerUserID int, referenceID string) (*model.StorageObject, error) {
	var object model.StorageObject
	err := liveVideoStudioObjects(model.DB, ownerUserID, common.GetTimestamp()).Where("object_id = ?", referenceID).First(&object).Error
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func PresignVideoStudioReference(ctx context.Context, ownerUserID int, referenceID string, thumbnail bool) (string, int64, error) {
	object, err := FindVideoStudioReference(ownerUserID, referenceID)
	if err != nil {
		return "", 0, err
	}
	return presignVideoStudioObject(ctx, object, thumbnail)
}

func presignVideoStudioObject(ctx context.Context, object *model.StorageObject, thumbnail bool) (string, int64, error) {
	policy, err := GetVideoStudioPolicy()
	if err != nil {
		return "", 0, err
	}
	profile, err := model.GetStorageProfileByID(object.StorageProfileID)
	if err != nil || profile == nil {
		return "", 0, &RequestError{StatusCode: http.StatusServiceUnavailable, Err: errors.New("reference storage profile is unavailable")}
	}
	credential, err := model.GetActiveStorageCredential(profile.ID)
	if err != nil || credential == nil {
		return "", 0, &RequestError{StatusCode: http.StatusServiceUnavailable, Err: errors.New("reference storage credential is unavailable")}
	}
	driver, err := driverForProfile(profile, credential)
	if err != nil {
		return "", 0, &RequestError{StatusCode: http.StatusServiceUnavailable, Err: err}
	}
	ttl := policy.SignedURLTTLSeconds
	// A temporary file must not stay reachable past its retention deadline.
	if object.DeleteAfter > 0 {
		ttl = min(ttl, object.DeleteAfter-common.GetTimestamp())
	}
	if ttl <= 0 {
		return "", 0, gorm.ErrRecordNotFound
	}
	options := ObjectGetOptions{}
	if thumbnail {
		options.Process = "image/resize,m_lfit,w_640,h_360"
	}
	value, err := driver.PresignGet(ctx, object.ObjectKey, time.Duration(ttl)*time.Second, options)
	if err != nil {
		return "", 0, &RequestError{StatusCode: http.StatusBadGateway, Err: err}
	}
	return value, common.GetTimestamp() + ttl, nil
}

// SaveVideoStudioReference moves a temporary reference into the user's
// library. Saving an already saved file is a no-op.
func SaveVideoStudioReference(ownerUserID int, referenceID string) (*VideoStudioReference, error) {
	return saveVideoStudioReference(model.DB, ownerUserID, referenceID)
}

func saveVideoStudioReference(tx *gorm.DB, ownerUserID int, referenceID string) (*VideoStudioReference, error) {
	now := common.GetTimestamp()
	result := tx.Model(&model.StorageObject{}).
		Where("owner_user_id = ? AND purpose = ? AND object_id = ? AND status = ?", ownerUserID, model.StorageObjectPurposeVideoStudio, referenceID, model.StorageObjectStatusUploaded).
		Where("delete_after > ?", now).
		Updates(map[string]any{"delete_after": 0, "updated_at": now})
	if result.Error != nil {
		return nil, result.Error
	}
	var object model.StorageObject
	if err := liveVideoStudioObjects(tx, ownerUserID, now).Where("object_id = ?", referenceID).First(&object).Error; err != nil {
		return nil, err
	}
	return videoStudioReferenceView(&object), nil
}

// DeleteVideoStudioReference removes a library file. Files still backing a
// Seedance asset must be removed through the asset library first.
func DeleteVideoStudioReference(ownerUserID int, referenceID string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var object model.StorageObject
		if err := liveVideoStudioObjects(tx, ownerUserID, common.GetTimestamp()).Where("object_id = ?", referenceID).First(&object).Error; err != nil {
			return err
		}
		var assets int64
		if err := tx.Model(&model.MediaAsset{}).Where("storage_object_id = ? AND status <> ?", object.ID, model.AssetStatusDeleted).Count(&assets).Error; err != nil {
			return err
		}
		if assets > 0 {
			return &RequestError{StatusCode: http.StatusConflict, Err: errors.New("reference is registered as a Seedance asset; delete the asset first")}
		}
		now := common.GetTimestamp()
		return tx.Model(&model.StorageObject{}).Where("id = ? AND status = ?", object.ID, model.StorageObjectStatusUploaded).Updates(map[string]any{
			"status": model.StorageObjectStatusDeletePending, "delete_after": now, "last_error": "deleted by owner", "updated_at": now,
		}).Error
	})
}

// AdoptVideoStudioReferenceForAsset marks the reference as a library file
// inside the asset registration transaction and returns its ledger row.
func AdoptVideoStudioReferenceForAsset(tx *gorm.DB, ownerUserID int, referenceID string) (*model.StorageObject, error) {
	if _, err := saveVideoStudioReference(tx, ownerUserID, referenceID); err != nil {
		return nil, err
	}
	var object model.StorageObject
	err := tx.Where("owner_user_id = ? AND purpose = ? AND object_id = ? AND status = ? AND delete_after = 0",
		ownerUserID, model.StorageObjectPurposeVideoStudio, referenceID, model.StorageObjectStatusUploaded).First(&object).Error
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func liveVideoStudioObjects(db *gorm.DB, ownerUserID int, now int64) *gorm.DB {
	return db.Model(&model.StorageObject{}).
		Where("owner_user_id = ? AND purpose = ? AND status = ?", ownerUserID, model.StorageObjectPurposeVideoStudio, model.StorageObjectStatusUploaded).
		Where("delete_after = 0 OR delete_after > ?", now)
}

func videoStudioReferenceView(object *model.StorageObject) *VideoStudioReference {
	return &VideoStudioReference{
		ID:          object.ObjectID,
		FileName:    object.FileName,
		ContentType: object.ContentType,
		Size:        object.Size,
		SHA256:      object.SHA256,
		Saved:       object.DeleteAfter == 0,
		ExpiresAt:   object.DeleteAfter,
		CreatedAt:   object.CreatedAt,
	}
}

func sanitizeReferenceFileName(fileName string, extension string) string {
	name := strings.TrimSpace(filepath.Base(strings.ReplaceAll(fileName, "\\", "/")))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`"/\`, r) {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "reference." + extension
	}
	for len(name) > 255 || !utf8.ValidString(name) {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-max(size, 1)]
	}
	return name
}

// DecodeImageDataURI decodes a base64 image data URI. Type checks happen on
// the decoded bytes in UploadVideoStudioReference.
func DecodeImageDataURI(dataURI string) ([]byte, error) {
	metadata, encoded, ok := strings.Cut(strings.TrimSpace(dataURI), ",")
	metadata = strings.ToLower(metadata)
	if !ok || !strings.HasPrefix(metadata, "data:image/") || !strings.HasSuffix(metadata, ";base64") {
		return nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("data_uri must be a base64 image data URI")}
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("data_uri contains invalid base64")}
	}
	return payload, nil
}
