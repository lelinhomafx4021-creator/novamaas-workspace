package model

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const SMSChallengeTTL = 5 * time.Minute
const SMSMaxAttempts = 5

var ErrSMSInvalid = errors.New("SMS verification is invalid or expired")
var ErrSMSRateLimit = errors.New("SMS send limit exceeded")

// SMSChallenge is a purpose-bound proof of phone possession. Neither the
// opaque challenge token nor the six-digit code is stored in plaintext.
type SMSChallenge struct {
	Id          int64  `gorm:"primaryKey"`
	TokenHash   string `gorm:"type:char(64);not null;uniqueIndex"`
	CodeHash    string `gorm:"type:char(64);not null"`
	Purpose     string `gorm:"type:varchar(32);not null;index"`
	Phone       string `gorm:"type:varchar(16);not null;index"`
	UserId      int    `gorm:"index"`
	AuthVersion int64
	SessionId   string `gorm:"type:varchar(64)"`
	FlowHash    string `gorm:"type:char(64)"`
	Attempts    int
	Delivered   bool
	CreatedAt   time.Time
	ExpiresAt   time.Time `gorm:"index"`
	ConsumedAt  *time.Time
}

// SMSBudget counters are shared by every process and by every SMS purpose.
// Conditional UPDATE protects limits even where row locks are unavailable.
type SMSBudget struct {
	BudgetHash string `gorm:"type:char(64);primaryKey"`
	Used       int
	NextAt     time.Time
	ExpiresAt  time.Time `gorm:"index"`
}

type SMSChallengeInput struct {
	Purpose       string
	Phone         string
	UserId        int
	AuthVersion   int64
	SessionId     string
	FlowToken     string
	IP            string
	NativeSubject string
}

func CanonicalMobilePhone(raw string) (string, error) {
	phone := strings.TrimSpace(raw)
	for _, prefix := range []string{"+86", "0086", "86"} {
		if national := strings.TrimSpace(strings.TrimPrefix(phone, prefix)); strings.HasPrefix(phone, prefix) && len(national) == 11 {
			phone = national
			break
		}
	}
	if len(phone) != 11 || phone[0] != '1' || phone[1] < '3' || phone[1] > '9' {
		return "", ErrVerifiedPhoneInvalid
	}
	for _, digit := range phone {
		if digit < '0' || digit > '9' {
			return "", ErrVerifiedPhoneInvalid
		}
	}
	return "+86" + phone, nil
}

func smsDigest(value string) string {
	return common.GenerateHMACWithKey([]byte("sms-v1:"+common.SessionSecret), value)
}

func ReserveSMSChallenge(input SMSChallengeInput) (string, string, *SMSChallenge, error) {
	phone, err := CanonicalMobilePhone(input.Phone)
	if err != nil {
		return "", "", nil, err
	}
	if input.Purpose != "login" && input.Purpose != "mfa_login" && input.Purpose != "phone_bind" && input.Purpose != "security" && input.Purpose != "user_mutation" {
		return "", "", nil, ErrSMSInvalid
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", "", nil, err
	}
	code := fmt.Sprintf("%06d", number.Int64())
	now := time.Now()
	challenge := &SMSChallenge{TokenHash: smsDigest("token:" + token), CodeHash: smsDigest(token + ":" + code),
		Purpose: input.Purpose, Phone: phone, UserId: input.UserId, AuthVersion: input.AuthVersion,
		SessionId: input.SessionId, FlowHash: smsDigest("flow:" + input.FlowToken), CreatedAt: now, ExpiresAt: now.Add(SMSChallengeTTL)}
	err = DB.Transaction(func(tx *gorm.DB) error {
		budgets := []struct {
			key      string
			limit    int
			duration time.Duration
		}{
			{"phone:" + phone, 5, time.Hour}, {"phone:" + phone, 10, 24 * time.Hour},
			{"ip:" + input.IP, 20, time.Hour}, {"ip:" + input.IP, 100, 24 * time.Hour},
			{"global", 100, time.Minute}, {"global", 5000, 24 * time.Hour},
		}
		if input.UserId > 0 {
			budgets = append(budgets, struct {
				key      string
				limit    int
				duration time.Duration
			}{fmt.Sprintf("user:%d", input.UserId), 30, 24 * time.Hour})
		}
		if input.NativeSubject != "" {
			budgets = append(budgets, struct {
				key      string
				limit    int
				duration time.Duration
			}{"native:" + input.NativeSubject, 30, 24 * time.Hour})
		}
		// Sort-independent fixed order is identical for every purpose. Phone
		// cooldown also prevents rapid retries across adjacent window buckets.
		cooldown := SMSBudget{BudgetHash: smsDigest("cooldown:" + phone), NextAt: now.Add(-time.Minute), ExpiresAt: now.Add(24 * time.Hour)}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&cooldown).Error; err != nil {
			return err
		}
		result := tx.Model(&SMSBudget{}).Where("budget_hash = ? AND next_at <= ?", cooldown.BudgetHash, now).
			Updates(map[string]any{"next_at": now.Add(time.Minute), "expires_at": now.Add(24 * time.Hour)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSMSRateLimit
		}
		for _, budget := range budgets {
			window := now.Truncate(budget.duration)
			row := SMSBudget{BudgetHash: smsDigest(fmt.Sprintf("budget:%s:%d:%d", budget.key, budget.duration, window.Unix())), NextAt: window, ExpiresAt: window.Add(budget.duration)}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
			result := tx.Model(&SMSBudget{}).Where("budget_hash = ? AND used < ?", row.BudgetHash, budget.limit).Update("used", gorm.Expr("used + 1"))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrSMSRateLimit
			}
		}
		if err := tx.Model(&SMSChallenge{}).Where("phone = ? AND purpose = ? AND user_id = ? AND session_id = ? AND consumed_at IS NULL", phone, input.Purpose, input.UserId, input.SessionId).Update("consumed_at", now).Error; err != nil {
			return err
		}
		return tx.Create(challenge).Error
	})
	if err != nil {
		return "", "", nil, err
	}
	return token, code, challenge, nil
}

func MarkSMSDelivered(id int64) error {
	result := DB.Model(&SMSChallenge{}).Where("id = ? AND consumed_at IS NULL AND expires_at > ?", id, time.Now()).Update("delivered", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSMSInvalid
	}
	return nil
}

func GetSMSChallenge(token, purpose string) (*SMSChallenge, error) {
	if len(token) != 43 {
		return nil, ErrSMSInvalid
	}
	var challenge SMSChallenge
	if err := DB.Where("token_hash = ? AND purpose = ?", smsDigest("token:"+token), purpose).First(&challenge).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSMSInvalid
		}
		return nil, err
	}
	return &challenge, nil
}

// ConsumeSMSChallenge commits failed attempts rather than rolling them back.
// A successful action and code consumption either commit together or neither.
func ConsumeSMSChallenge(token, code string, match SMSChallengeInput, action func(*gorm.DB, *SMSChallenge) error) (*SMSChallenge, error) {
	if len(token) != 43 || len(code) != 6 {
		return nil, ErrSMSInvalid
	}
	if match.Phone != "" {
		phone, err := CanonicalMobilePhone(match.Phone)
		if err != nil {
			return nil, ErrSMSInvalid
		}
		match.Phone = phone
	}
	var challenge SMSChallenge
	var rejected error
	err := DB.Transaction(func(tx *gorm.DB) error {
		query := lockForUpdate(tx).Where("token_hash = ? AND purpose = ? AND user_id = ? AND session_id = ? AND flow_hash = ?", smsDigest("token:"+token), match.Purpose, match.UserId, match.SessionId, smsDigest("flow:"+match.FlowToken))
		if err := query.First(&challenge).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				rejected = ErrSMSInvalid
				return nil
			}
			return err
		}
		now := time.Now()
		if challenge.ConsumedAt != nil || !challenge.Delivered || !challenge.ExpiresAt.After(now) || challenge.Attempts >= SMSMaxAttempts {
			rejected = ErrSMSInvalid
			return nil
		}
		result := tx.Model(&SMSChallenge{}).Where("id = ? AND attempts = ? AND attempts < ? AND consumed_at IS NULL", challenge.Id, challenge.Attempts, SMSMaxAttempts).Update("attempts", gorm.Expr("attempts + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			rejected = ErrSMSInvalid
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(challenge.CodeHash), []byte(smsDigest(token+":"+code))) != 1 {
			rejected = ErrSMSInvalid
			return nil
		}
		if match.Phone != "" && match.Phone != challenge.Phone {
			rejected = ErrSMSInvalid
			return nil
		}
		if challenge.UserId > 0 {
			var user User
			if err := lockForUpdate(tx).Where("id = ? AND status = ? AND auth_version = ?", challenge.UserId, common.UserStatusEnabled, challenge.AuthVersion).First(&user).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					rejected = ErrSMSInvalid
					return nil
				}
				return err
			}
		}
		if action != nil {
			if err := action(tx, &challenge); err != nil {
				return err
			}
		}
		result = tx.Model(&SMSChallenge{}).Where("id = ? AND consumed_at IS NULL", challenge.Id).Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSMSInvalid
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if rejected != nil {
		return nil, rejected
	}
	return &challenge, nil
}

func CleanupSMSChallenges(now time.Time) error {
	if err := DB.Where("expires_at < ?", now.Add(-24*time.Hour)).Delete(&SMSChallenge{}).Error; err != nil {
		return err
	}
	return DB.Where("expires_at < ?", now).Delete(&SMSBudget{}).Error
}
