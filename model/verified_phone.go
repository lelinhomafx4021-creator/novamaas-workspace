package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrVerifiedPhoneAlreadyClaimed = errors.New("verified phone is already claimed")
var ErrVerifiedPhoneInvalid = errors.New("verified phone is invalid")
var ErrVerifiedPhoneProtected = errors.New("Use phone verification to change a bound phone number")

// EnsureVerifiedPhoneUnchangedWithTx prevents generic profile mutations from
// changing a login identity without going through the phone verification flow.
func EnsureVerifiedPhoneUnchangedWithTx(tx *gorm.DB, userId int, phone string) error {
	var user User
	if err := lockForUpdate(tx).Select("id").Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}
	var claim VerifiedPhone
	err := lockForUpdate(tx).Where("user_id = ?", userId).First(&claim).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	canonical, err := CanonicalMobilePhone(phone)
	if err != nil || canonical != claim.Phone {
		return ErrVerifiedPhoneProtected
	}
	return nil
}

// VerifiedPhone owns the phone-login identity. System-assigned users.phone can
// be trusted as a provisional owner only after WeChat proves possession.
type VerifiedPhone struct {
	Id         int64     `json:"id" gorm:"primaryKey"`
	Phone      string    `json:"phone" gorm:"type:varchar(16);not null;uniqueIndex"`
	UserId     int       `json:"user_id" gorm:"not null;uniqueIndex"`
	Source     string    `json:"source" gorm:"type:varchar(32);not null"`
	VerifiedAt time.Time `json:"verified_at" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at"`
}

func (VerifiedPhone) TableName() string {
	return "verified_phones"
}

// WeChatVerifiedPhone converts provider country/national fields to one
// canonical E.164-like identity before looking up or claiming ownership.
func WeChatVerifiedPhone(countryCode, nationalNumber string) (string, error) {
	countryCode = strings.TrimSpace(countryCode)
	nationalNumber = strings.TrimSpace(nationalNumber)
	if len(countryCode) < 1 || len(countryCode) > 3 || len(nationalNumber) < 4 {
		return "", ErrVerifiedPhoneInvalid
	}
	phone := "+" + countryCode + nationalNumber
	if len(phone) < 9 || len(phone) > 16 {
		return "", ErrVerifiedPhoneInvalid
	}
	for _, digit := range phone[1:] {
		if digit < '0' || digit > '9' {
			return "", ErrVerifiedPhoneInvalid
		}
	}
	if phone[1] == '0' {
		return "", ErrVerifiedPhoneInvalid
	}
	return phone, nil
}

// ClaimVerifiedPhoneWithTx is idempotent only for the same user and number.
// Unique indexes protect both directions on SQLite, MySQL and PostgreSQL.
func ClaimVerifiedPhoneWithTx(tx *gorm.DB, phone string, userId int, source string) error {
	phone = strings.TrimSpace(phone)
	if tx == nil || len(phone) < 9 || len(phone) > 16 || phone[0] != '+' || phone[1] == '0' || userId <= 0 || strings.TrimSpace(source) == "" {
		return ErrVerifiedPhoneInvalid
	}
	for _, digit := range phone[1:] {
		if digit < '0' || digit > '9' {
			return ErrVerifiedPhoneInvalid
		}
	}
	assignedOwner, err := GetTrustedPhoneOwnerWithTx(tx, phone)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if assignedOwner != nil && assignedOwner.Id != userId {
		return ErrVerifiedPhoneAlreadyClaimed
	}
	claim := VerifiedPhone{Phone: phone, UserId: userId, Source: source, VerifiedAt: time.Now()}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim).Error; err != nil {
		return err
	}
	var owner VerifiedPhone
	if err := tx.Where("phone = ?", phone).First(&owner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrVerifiedPhoneAlreadyClaimed
		}
		return err
	}
	if owner.UserId != userId {
		return ErrVerifiedPhoneAlreadyClaimed
	}
	var userPhone VerifiedPhone
	if err := tx.Where("user_id = ?", userId).First(&userPhone).Error; err != nil {
		return err
	}
	if userPhone.Phone != phone {
		return ErrVerifiedPhoneAlreadyClaimed
	}
	return nil
}

// GetTrustedPhoneOwnerWithTx resolves an already verified claim or a unique
// system-assigned users.phone value. For Chinese numbers, the stored national,
// 86-prefixed and E.164 forms are equivalent. Ambiguous ownership fails shut.
func GetTrustedPhoneOwnerWithTx(tx *gorm.DB, phone string) (*User, error) {
	phone = strings.TrimSpace(phone)
	if tx == nil || len(phone) < 9 || len(phone) > 16 || phone[0] != '+' {
		return nil, ErrVerifiedPhoneInvalid
	}
	var verified VerifiedPhone
	verifiedErr := lockForUpdate(tx).Where("phone = ?", phone).First(&verified).Error
	if verifiedErr != nil && !errors.Is(verifiedErr, gorm.ErrRecordNotFound) {
		return nil, verifiedErr
	}
	candidates := []string{phone}
	if strings.HasPrefix(phone, "+86") && len(phone) == 14 {
		national := strings.TrimPrefix(phone, "+86")
		candidates = append(candidates, national, "86"+national, "0086"+national)
	}
	var assigned []User
	if err := lockForUpdate(tx).Where("phone IN ?", candidates).Limit(2).Find(&assigned).Error; err != nil {
		return nil, err
	}
	if len(assigned) > 1 || (len(assigned) == 1 && verifiedErr == nil && assigned[0].Id != verified.UserId) {
		return nil, ErrVerifiedPhoneAlreadyClaimed
	}
	if verifiedErr == nil {
		if len(assigned) == 1 {
			return &assigned[0], nil
		}
		var owner User
		if err := lockForUpdate(tx).Where("id = ?", verified.UserId).First(&owner).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrVerifiedPhoneAlreadyClaimed
			}
			return nil, err
		}
		return &owner, nil
	}
	if len(assigned) == 1 {
		return &assigned[0], nil
	}
	return nil, gorm.ErrRecordNotFound
}

func GetVerifiedPhoneByUser(userId int) (*VerifiedPhone, error) {
	var claim VerifiedPhone
	if err := DB.Where("user_id = ?", userId).First(&claim).Error; err != nil {
		return nil, err
	}
	return &claim, nil
}

func GetUserIdByVerifiedPhone(phone string) (int, error) {
	var claim VerifiedPhone
	if err := DB.Where("phone = ?", strings.TrimSpace(phone)).First(&claim).Error; err != nil {
		return 0, err
	}
	return claim.UserId, nil
}

// EditWithPhoneVerificationWithTx is used only after a purpose-bound SMS code
// has been checked in this same transaction. The old claim, user fields and
// credential epoch change together, including verification of a legacy number.
func (user *User) EditWithPhoneVerificationWithTx(tx *gorm.DB, updatePassword bool, expectedAuthVersion int64) error {
	var current User
	if err := lockForUpdate(tx).Where("id = ?", user.Id).First(&current).Error; err != nil {
		return err
	}
	if current.AuthVersion != expectedAuthVersion {
		return ErrSMSInvalid
	}
	if err := tx.Where("user_id = ?", user.Id).Delete(&VerifiedPhone{}).Error; err != nil {
		return err
	}
	if err := ClaimVerifiedPhoneWithTx(tx, user.Phone, user.Id, "aliyun_sms"); err != nil {
		return err
	}
	if err := user.EditWithTx(tx, updatePassword); err != nil {
		return err
	}
	if user.AuthVersion == expectedAuthVersion {
		version, err := IncrementUserAuthVersionWithTx(tx, user.Id)
		if err != nil {
			return err
		}
		user.AuthVersion = version
	}
	return nil
}
