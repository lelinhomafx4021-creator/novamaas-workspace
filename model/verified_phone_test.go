package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestVerifiedPhoneClaimRespectsSystemAssignedPhoneOwner(t *testing.T) {
	truncateTables(t)
	legacy := User{Username: "legacy-phone-owner", Password: "password", Phone: "13800138000", AffCode: "legacy-phone-owner"}
	verified := User{Username: "verified-phone-owner", Password: "password", AffCode: "verified-phone-owner"}
	require.NoError(t, DB.Create(&legacy).Error)
	require.NoError(t, DB.Create(&verified).Error)

	_, err := GetUserIdByVerifiedPhone("+8613800138000")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ClaimVerifiedPhoneWithTx(tx, "+8613800138000", verified.Id, "wechat_miniapp")
	})
	assert.ErrorIs(t, err, ErrVerifiedPhoneAlreadyClaimed)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimVerifiedPhoneWithTx(tx, "+8613800138000", legacy.Id, "wechat_miniapp")
	}))
	owner, err := GetUserIdByVerifiedPhone("+8613800138000")
	require.NoError(t, err)
	assert.Equal(t, legacy.Id, owner)
}

func TestTrustedPhoneOwnerRejectsAmbiguousAssignedFormats(t *testing.T) {
	truncateTables(t)
	first := User{Username: "phone-national", Password: "password", Phone: "13800138000", AffCode: "phone-national"}
	second := User{Username: "phone-e164", Password: "password", Phone: "+8613800138000", AffCode: "phone-e164"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)
	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := GetTrustedPhoneOwnerWithTx(tx, "+8613800138000")
		return err
	})
	assert.ErrorIs(t, err, ErrVerifiedPhoneAlreadyClaimed)
}

func TestVerifiedPhoneClaimRejectsCompetingNumberOrUser(t *testing.T) {
	truncateTables(t)
	first := User{Username: "verified-one", Password: "password", AffCode: "verified-one"}
	second := User{Username: "verified-two", Password: "password", AffCode: "verified-two"}
	require.NoError(t, DB.Create(&first).Error)
	require.NoError(t, DB.Create(&second).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimVerifiedPhoneWithTx(tx, "+8613800138000", first.Id, "wechat_miniapp")
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ClaimVerifiedPhoneWithTx(tx, "+8613800138000", first.Id, "wechat_miniapp")
	}))

	err := DB.Transaction(func(tx *gorm.DB) error {
		return ClaimVerifiedPhoneWithTx(tx, "+8613800138000", second.Id, "wechat_miniapp")
	})
	assert.ErrorIs(t, err, ErrVerifiedPhoneAlreadyClaimed)

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ClaimVerifiedPhoneWithTx(tx, "+8613900139000", first.Id, "wechat_miniapp")
	})
	assert.ErrorIs(t, err, ErrVerifiedPhoneAlreadyClaimed)

	var count int64
	require.NoError(t, DB.Model(&VerifiedPhone{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestWeChatVerifiedPhoneRejectsMalformedProviderNumbers(t *testing.T) {
	tests := []struct {
		country  string
		national string
		want     string
		valid    bool
	}{
		{country: "86", national: "13800138000", want: "+8613800138000", valid: true},
		{country: "1", national: "2125551234", want: "+12125551234", valid: true},
		{country: "0", national: "13800138000"},
		{country: "86", national: "138-0013-8000"},
		{country: "86", national: "123"},
	}
	for _, test := range tests {
		t.Run(test.country+"-"+test.national, func(t *testing.T) {
			phone, err := WeChatVerifiedPhone(test.country, test.national)
			if test.valid {
				require.NoError(t, err)
				assert.Equal(t, test.want, phone)
				return
			}
			assert.ErrorIs(t, err, ErrVerifiedPhoneInvalid)
		})
	}
}
