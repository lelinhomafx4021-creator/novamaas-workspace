package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ExternalIdentityProviderTelegram      = "telegram"
	ExternalIdentityProviderWeChatMiniApp = "wechat_miniapp"
)

var ErrExternalIdentityAlreadyClaimed = errors.New("external identity is already claimed")

// ExternalIdentityClaim is the durable ownership record for an identity issued
// by an external provider. The two unique indexes make both the provider
// subject and the user's provider/app slot single-owner without relying on a
// check-then-update sequence. AppId is empty for providers whose identities
// are not scoped to an application, such as the legacy Telegram integration.
type ExternalIdentityClaim struct {
	Id        int64     `json:"id" gorm:"primaryKey"`
	Provider  string    `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:idx_external_identity_subject_v2,priority:1;uniqueIndex:idx_external_identity_user_v2,priority:1"`
	AppId     string    `json:"app_id" gorm:"column:app_id;type:varchar(64);not null;default:'';uniqueIndex:idx_external_identity_subject_v2,priority:2;uniqueIndex:idx_external_identity_user_v2,priority:2"`
	Subject   string    `json:"subject" gorm:"type:varchar(128);not null;uniqueIndex:idx_external_identity_subject_v2,priority:3"`
	UserId    int       `json:"user_id" gorm:"not null;index;uniqueIndex:idx_external_identity_user_v2,priority:3"`
	CreatedAt time.Time `json:"created_at"`
}

func (ExternalIdentityClaim) TableName() string {
	return "external_identity_claims"
}

// ClaimExternalIdentityWithTx atomically claims a provider subject for one
// user. Repeating the exact mapping is idempotent; every competing subject or
// user is rejected. Ownership is read back instead of trusting RowsAffected,
// whose duplicate-key semantics differ between supported databases.
func ClaimExternalIdentityWithTx(tx *gorm.DB, provider, subject string, userId int) error {
	return ClaimScopedExternalIdentityWithTx(tx, provider, "", subject, userId)
}

// ClaimScopedExternalIdentityWithTx claims one provider/app subject for a
// user. Repeating the exact mapping is idempotent; a competing subject owner
// or a second subject for the same user's provider/app slot is rejected.
func ClaimScopedExternalIdentityWithTx(tx *gorm.DB, provider, appId, subject string, userId int) error {
	provider = strings.TrimSpace(provider)
	appId = strings.TrimSpace(appId)
	subject = strings.TrimSpace(subject)
	if tx == nil || provider == "" || subject == "" || userId == 0 {
		return errors.New("external identity claim is invalid")
	}

	claim := ExternalIdentityClaim{Provider: provider, AppId: appId, Subject: subject, UserId: userId}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
	if result.Error != nil {
		return result.Error
	}
	var subjectOwner ExternalIdentityClaim
	if err := tx.Where("provider = ? AND app_id = ? AND subject = ?", provider, appId, subject).First(&subjectOwner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExternalIdentityAlreadyClaimed
		}
		return err
	}
	if subjectOwner.UserId != userId {
		return ErrExternalIdentityAlreadyClaimed
	}

	var userClaim ExternalIdentityClaim
	if err := tx.Where("provider = ? AND app_id = ? AND user_id = ?", provider, appId, userId).First(&userClaim).Error; err != nil {
		return err
	}
	if userClaim.Subject != subject {
		return ErrExternalIdentityAlreadyClaimed
	}
	return nil
}

func GetUserIdByScopedExternalIdentity(provider, appId, subject string) (int, error) {
	provider = strings.TrimSpace(provider)
	appId = strings.TrimSpace(appId)
	subject = strings.TrimSpace(subject)
	if provider == "" || subject == "" {
		return 0, errors.New("external identity lookup is invalid")
	}
	var claim ExternalIdentityClaim
	if err := DB.Where("provider = ? AND app_id = ? AND subject = ?", provider, appId, subject).First(&claim).Error; err != nil {
		return 0, err
	}
	return claim.UserId, nil
}

// MigrateExternalIdentityClaimIndexes removes pre-AppId unique indexes after
// AutoMigrate has created the v2 indexes. GORM's migrator implements this for
// SQLite, MySQL and PostgreSQL without dialect-specific SQL.
func MigrateExternalIdentityClaimIndexes() error {
	if DB == nil || !DB.Migrator().HasTable(&ExternalIdentityClaim{}) {
		return nil
	}
	for _, legacyIndex := range []string{"idx_external_identity_subject", "idx_external_identity_user"} {
		if DB.Migrator().HasIndex(&ExternalIdentityClaim{}, legacyIndex) {
			if err := DB.Migrator().DropIndex(&ExternalIdentityClaim{}, legacyIndex); err != nil {
				return err
			}
		}
	}
	return nil
}

func ReleaseExternalIdentityWithTx(tx *gorm.DB, provider string, userId int) error {
	provider = strings.TrimSpace(provider)
	if tx == nil || provider == "" || userId == 0 {
		return errors.New("external identity release is invalid")
	}
	return tx.Where("provider = ? AND user_id = ?", provider, userId).
		Delete(&ExternalIdentityClaim{}).Error
}

func releaseAllExternalIdentitiesWithTx(tx *gorm.DB, userId int) error {
	if tx == nil || userId == 0 {
		return errors.New("external identity release is invalid")
	}
	return tx.Where("user_id = ?", userId).Delete(&ExternalIdentityClaim{}).Error
}

// InitializeExternalIdentityClaims imports legacy Telegram bindings after the
// claim table is migrated. Existing duplicate ownership fails migration rather
// than preserving an ambiguous login identity.
func InitializeExternalIdentityClaims() error {
	var users []User
	if err := DB.Unscoped().Select("id", "telegram_id").
		Where("telegram_id <> ?", "").Find(&users).Error; err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, user := range users {
			if err := ClaimExternalIdentityWithTx(tx, ExternalIdentityProviderTelegram, user.TelegramId, user.Id); err != nil {
				return fmt.Errorf("backfill Telegram identity for user %d: %w", user.Id, err)
			}
		}
		return nil
	})
}
