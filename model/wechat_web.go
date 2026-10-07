package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrWeChatWebBindingRequired = errors.New("Link this WeChat account in the mini program before signing in")

func GetWeChatWebLoginUser(unionId string) (*User, error) {
	if unionId == "" {
		return nil, ErrWeChatWebBindingRequired
	}
	var user User
	err := DB.Transaction(func(tx *gorm.DB) error {
		var profiles []WeChatMiniAppProfile
		if err := tx.Omit("avatar").Where("union_id = ?", unionId).Find(&profiles).Error; err != nil {
			return err
		}
		userId := 0
		for _, profile := range profiles {
			var count int64
			if err := tx.Model(&ExternalIdentityClaim{}).Where("provider = ? AND app_id = ? AND user_id = ?", ExternalIdentityProviderWeChatMiniApp, profile.AppId, profile.UserId).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				continue
			}
			if userId != 0 && userId != profile.UserId {
				return ErrWeChatWebBindingRequired
			}
			userId = profile.UserId
		}
		if userId == 0 {
			return ErrWeChatWebBindingRequired
		}
		if err := lockForUpdate(tx).Where("id = ? AND status = ?", userId, common.UserStatusEnabled).First(&user).Error; err != nil {
			return err
		}
		// Recheck after the account lock: unbinding uses this same lock.
		var count int64
		if err := tx.Model(&WeChatMiniAppProfile{}).
			Joins("JOIN external_identity_claims ON external_identity_claims.user_id = we_chat_mini_app_profiles.user_id AND external_identity_claims.app_id = we_chat_mini_app_profiles.app_id").
			Where("we_chat_mini_app_profiles.user_id = ? AND we_chat_mini_app_profiles.union_id = ? AND external_identity_claims.provider = ?", userId, unionId, ExternalIdentityProviderWeChatMiniApp).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrWeChatWebBindingRequired
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}
