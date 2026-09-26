package assetlibrary

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const maxAssetAccessKeysPerUser = 5

type AccessKeyView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	AccessKeyID string `json:"access_key_id"`
	SecretHint  string `json:"secret_hint"`
	Status      string `json:"status"`
	LastUsedAt  int64  `json:"last_used_at"`
	CreatedAt   int64  `json:"created_at"`
}

type CreatedAccessKeyView struct {
	AccessKeyView
	SecretAccessKey string `json:"secret_access_key"`
}

func ListAccessKeys(ownerUserID int) ([]AccessKeyView, error) {
	var keys []model.AssetAccessKey
	if err := model.DB.Where("owner_user_id = ? AND status = ?", ownerUserID, model.AssetAccessKeyStatusEnabled).
		Order("created_at desc").Find(&keys).Error; err != nil {
		return nil, err
	}
	views := make([]AccessKeyView, 0, len(keys))
	for _, key := range keys {
		views = append(views, accessKeyView(key))
	}
	return views, nil
}

func CreateAccessKey(ownerUserID int, name string) (*CreatedAccessKeyView, error) {
	name = strings.TrimSpace(name)
	if ownerUserID <= 0 {
		return nil, &RequestError{StatusCode: http.StatusUnauthorized, Err: errors.New("authentication is required")}
	}
	if name == "" || len([]rune(name)) > 64 {
		return nil, &RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("access key name must contain 1 to 64 characters")}
	}
	var count int64
	if err := model.DB.Model(&model.AssetAccessKey{}).
		Where("owner_user_id = ? AND status = ?", ownerUserID, model.AssetAccessKeyStatusEnabled).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= maxAssetAccessKeysPerUser {
		return nil, &RequestError{StatusCode: http.StatusConflict, Err: errors.New("a user can have at most 5 active asset access keys")}
	}

	accessKeyRandom, err := common.GenerateRandomCharsKey(20)
	if err != nil {
		return nil, err
	}
	secret, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	encryptedSecret, err := encryptAccessKeySecret(secret)
	if err != nil {
		return nil, &RequestError{StatusCode: http.StatusServiceUnavailable, Err: err}
	}
	key := model.AssetAccessKey{
		OwnerUserID:          ownerUserID,
		Name:                 name,
		AccessKeyID:          "AKNM" + accessKeyRandom,
		SecretHint:           credentialHint(secret),
		EncryptedSecret:      encryptedSecret,
		CredentialKeyVersion: accessKeySecretVersion,
		Status:               model.AssetAccessKeyStatusEnabled,
	}
	if err = model.DB.Create(&key).Error; err != nil {
		return nil, err
	}
	return &CreatedAccessKeyView{AccessKeyView: accessKeyView(key), SecretAccessKey: secret}, nil
}

func DeleteAccessKey(ownerUserID int, id int64) error {
	result := model.DB.Model(&model.AssetAccessKey{}).
		Where("id = ? AND owner_user_id = ? AND status = ?", id, ownerUserID, model.AssetAccessKeyStatusEnabled).
		Updates(map[string]any{"status": model.AssetAccessKeyStatusDisabled, "updated_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func accessKeyView(key model.AssetAccessKey) AccessKeyView {
	return AccessKeyView{
		ID: key.ID, Name: key.Name, AccessKeyID: key.AccessKeyID, SecretHint: key.SecretHint,
		Status: key.Status, LastUsedAt: key.LastUsedAt, CreatedAt: key.CreatedAt,
	}
}
