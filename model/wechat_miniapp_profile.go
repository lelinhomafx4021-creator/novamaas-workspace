package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The claim table remains the identity authority. This table holds optional
// WeChat profile data; nicknames and avatars never establish ownership.
type WeChatMiniAppProfile struct {
	Id             int64      `json:"-" gorm:"primaryKey"`
	UserId         int        `json:"-" gorm:"uniqueIndex:idx_mini_profile_user_app,priority:1"`
	AppId          string     `json:"app_id" gorm:"type:varchar(64);uniqueIndex:idx_mini_profile_user_app,priority:2"`
	UnionId        string     `json:"-" gorm:"type:varchar(128)"`
	Nickname       string     `json:"nickname" gorm:"type:varchar(128)"`
	Avatar         []byte     `json:"-"`
	ProfileSource  string     `json:"profile_source" gorm:"type:varchar(32)"`
	PhoneAtBinding string     `json:"-" gorm:"type:varchar(16)"`
	ConsentVersion string     `json:"consent_version" gorm:"type:varchar(32)"`
	BoundAt        time.Time  `json:"bound_at"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type WeChatBindingView struct {
	AppId          string     `json:"app_id"`
	OpenId         string     `json:"openid,omitempty"`
	UnionId        string     `json:"unionid,omitempty"`
	Nickname       string     `json:"nickname"`
	HasAvatar      bool       `json:"has_avatar"`
	BoundAt        time.Time  `json:"bound_at"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ProfileSource  string     `json:"profile_source"`
	ConsentVersion string     `json:"consent_version"`
}

func RecordWeChatBindingWithTx(tx *gorm.DB, userId int, appId, unionId, phone string) error {
	var claim ExternalIdentityClaim
	if err := tx.Where("provider = ? AND app_id = ? AND user_id = ?", ExternalIdentityProviderWeChatMiniApp, appId, userId).First(&claim).Error; err != nil {
		return err
	}
	profile := WeChatMiniAppProfile{UserId: userId, AppId: appId, UnionId: unionId, PhoneAtBinding: phone,
		ProfileSource: "pending", ConsentVersion: "account-link-v1", BoundAt: claim.CreatedAt}
	if phone != "" {
		profile.ConsentVersion = "phone-link-v1"
	}
	// A repeated binding cannot overwrite a user's profile or consent history.
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&profile).Error
}

func GetWeChatBindings(userId int, admin bool) ([]WeChatBindingView, error) {
	var claims []ExternalIdentityClaim
	if err := DB.Where("provider = ? AND user_id = ?", ExternalIdentityProviderWeChatMiniApp, userId).Find(&claims).Error; err != nil {
		return nil, err
	}
	var profiles []WeChatMiniAppProfile
	if len(claims) > 0 {
		if err := DB.Omit("avatar").Where("user_id = ?", userId).Find(&profiles).Error; err != nil {
			return nil, err
		}
	}
	indexed := make(map[string]WeChatMiniAppProfile, len(profiles))
	for _, profile := range profiles {
		indexed[profile.AppId] = profile
	}
	// Select a boolean instead of materializing image blobs for summaries.
	var avatars []string
	if len(claims) > 0 {
		if err := DB.Model(&WeChatMiniAppProfile{}).Where("user_id = ? AND avatar IS NOT NULL", userId).Pluck("app_id", &avatars).Error; err != nil {
			return nil, err
		}
	}
	avatarSet := make(map[string]bool, len(avatars))
	for _, appId := range avatars {
		avatarSet[appId] = true
	}
	views := make([]WeChatBindingView, 0, len(claims))
	for _, claim := range claims {
		profile := indexed[claim.AppId]
		view := WeChatBindingView{AppId: claim.AppId, Nickname: profile.Nickname, HasAvatar: avatarSet[claim.AppId],
			BoundAt: claim.CreatedAt, LastLoginAt: profile.LastLoginAt, UpdatedAt: profile.UpdatedAt,
			ProfileSource: profile.ProfileSource, ConsentVersion: profile.ConsentVersion}
		if admin {
			view.OpenId, view.UnionId = claim.Subject, profile.UnionId
		}
		views = append(views, view)
	}
	return views, nil
}

// GetWeChatLoginUser captures the account version with the claim. Issuing a
// session at this version prevents an in-flight login from surviving unlink.
func GetWeChatLoginUser(appId, openId, phone string) (*User, error) {
	var user User
	err := DB.Transaction(func(tx *gorm.DB) error {
		var claim ExternalIdentityClaim
		if err := tx.Where("provider = ? AND app_id = ? AND subject = ?", ExternalIdentityProviderWeChatMiniApp, appId, openId).First(&claim).Error; err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where("id = ?", claim.UserId).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return ErrWeChatAccountUnavailable
		}
		if err := lockForUpdate(tx).Where("provider = ? AND app_id = ? AND subject = ? AND user_id = ?", ExternalIdentityProviderWeChatMiniApp, appId, openId, user.Id).First(&claim).Error; err != nil {
			return err
		}
		if phone != "" {
			var verified VerifiedPhone
			if err := tx.Where("user_id = ? AND phone = ?", user.Id, phone).First(&verified).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrVerifiedPhoneAlreadyClaimed
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

var ErrWeChatAccountUnavailable = errors.New("WeChat account is unavailable")

func RecordWeChatLogin(userId int, version int64, appId, openId, unionId string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := ValidateUserAuthVersionWithTx(tx, userId, version); err != nil {
			return err
		}
		var claim ExternalIdentityClaim
		if err := tx.Where("provider = ? AND app_id = ? AND subject = ? AND user_id = ?", ExternalIdentityProviderWeChatMiniApp, appId, openId, userId).First(&claim).Error; err != nil {
			return err
		}
		profile := WeChatMiniAppProfile{UserId: userId, AppId: appId, UnionId: unionId, ProfileSource: "pending", ConsentVersion: "legacy-unspecified", BoundAt: claim.CreatedAt}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&profile).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{"last_login_at": time.Now()}
		if unionId != "" {
			updates["union_id"] = unionId
		}
		return tx.Model(&WeChatMiniAppProfile{}).Where("user_id = ? AND app_id = ?", userId, appId).Updates(updates).Error
	})
}

// ValidateUserAuthVersionWithTx serializes credential changes against binding,
// profile updates and login ceremonies using the same account lock.
func ValidateUserAuthVersionWithTx(tx *gorm.DB, userId int, version int64) error {
	var user User
	if err := lockForUpdate(tx).Where("id = ? AND status = ?", userId, common.UserStatusEnabled).First(&user).Error; err != nil {
		return err
	}
	if user.AuthVersion != version {
		return ErrUserAuthVersionConflict
	}
	return nil
}
