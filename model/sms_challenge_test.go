package model

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSMSModelTest(t *testing.T) {
	t.Helper()
	previous, secret, dialect := DB, common.SessionSecret, common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SMSChallenge{}, &SMSBudget{}, &User{}, &VerifiedPhone{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.SessionSecret = "SMS-model-regression-test"
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previous
		common.SessionSecret = secret
		common.SetMainDatabaseType(dialect)
		require.NoError(t, sqlDB.Close())
	})
}

func TestCanonicalMobilePhone(t *testing.T) {
	for _, input := range []string{"13800138000", " +8613800138000 ", "8613800138000", "008613800138000", "+86 13800138000", "0086 13800138000", "86 13800138000"} {
		phone, err := CanonicalMobilePhone(input)
		require.NoError(t, err)
		assert.Equal(t, "+8613800138000", phone)
	}
	for _, input := range []string{"+12125551234", "138-0013-8000", "12345678901", "138001380000", "13800138abc", "+86 138 00138000"} {
		_, err := CanonicalMobilePhone(input)
		assert.ErrorIs(t, err, ErrVerifiedPhoneInvalid)
	}
}

func TestSMSChallengeRequiresDeliveryAndRejectsReplay(t *testing.T) {
	setupSMSModelTest(t)
	input := SMSChallengeInput{Purpose: "login", Phone: "13800138000", IP: "127.0.0.1"}
	token, code, challenge, err := ReserveSMSChallenge(input)
	require.NoError(t, err)
	assert.NotEqual(t, code, challenge.CodeHash)
	assert.Len(t, challenge.CodeHash, 64)
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	assert.ErrorIs(t, err, ErrSMSInvalid)
	require.NoError(t, MarkSMSDelivered(challenge.Id))
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	require.NoError(t, err)
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	assert.ErrorIs(t, err, ErrSMSInvalid)
}

func TestSMSChallengeFailedAttemptsPersist(t *testing.T) {
	setupSMSModelTest(t)
	input := SMSChallengeInput{Purpose: "login", Phone: "13800138000", IP: "127.0.0.1"}
	token, code, challenge, err := ReserveSMSChallenge(input)
	require.NoError(t, err)
	require.NoError(t, MarkSMSDelivered(challenge.Id))
	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}
	for i := 0; i < SMSMaxAttempts; i++ {
		_, err = ConsumeSMSChallenge(token, wrong, input, nil)
		require.ErrorIs(t, err, ErrSMSInvalid)
	}
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	assert.ErrorIs(t, err, ErrSMSInvalid)
	require.NoError(t, DB.First(challenge, challenge.Id).Error)
	assert.Equal(t, 5, challenge.Attempts)
}

func TestSMSChallengeCannotCrossPurposeSessionOrFlow(t *testing.T) {
	setupSMSModelTest(t)
	input := SMSChallengeInput{Purpose: "security", Phone: "13800138000", SessionId: "session-A", FlowToken: "flow-A", IP: "127.0.0.1"}
	token, code, challenge, err := ReserveSMSChallenge(input)
	require.NoError(t, err)
	require.NoError(t, MarkSMSDelivered(challenge.Id))
	for _, match := range []SMSChallengeInput{{Purpose: "login", SessionId: "session-A", FlowToken: "flow-A"}, {Purpose: "security", SessionId: "session-B", FlowToken: "flow-A"}, {Purpose: "security", SessionId: "session-A", FlowToken: "flow-B"}, {Purpose: "security", UserId: 42, SessionId: "session-A", FlowToken: "flow-A"}} {
		_, err := ConsumeSMSChallenge(token, code, match, nil)
		assert.ErrorIs(t, err, ErrSMSInvalid)
	}
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	require.NoError(t, err)
}

func TestSMSChallengeExpiredAndResentCodesFail(t *testing.T) {
	setupSMSModelTest(t)
	input := SMSChallengeInput{Purpose: "login", Phone: "13800138000", IP: "127.0.0.1"}
	token, code, challenge, err := ReserveSMSChallenge(input)
	require.NoError(t, err)
	require.NoError(t, MarkSMSDelivered(challenge.Id))
	require.NoError(t, DB.Model(challenge).Update("expires_at", time.Now().Add(-time.Second)).Error)
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	assert.ErrorIs(t, err, ErrSMSInvalid)
	require.NoError(t, DB.Model(&SMSBudget{}).Where("budget_hash = ?", smsDigest("cooldown:+8613800138000")).Update("next_at", time.Now().Add(-time.Second)).Error)
	newToken, newCode, next, err := ReserveSMSChallenge(input)
	require.NoError(t, err)
	require.NoError(t, MarkSMSDelivered(next.Id))
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	assert.ErrorIs(t, err, ErrSMSInvalid)
	_, err = ConsumeSMSChallenge(newToken, newCode, input, nil)
	require.NoError(t, err)
}

func TestSMSCooldownAppliesAcrossPurposes(t *testing.T) {
	setupSMSModelTest(t)
	_, _, _, err := ReserveSMSChallenge(SMSChallengeInput{Purpose: "login", Phone: "13800138000", IP: "127.0.0.1"})
	require.NoError(t, err)
	_, _, _, err = ReserveSMSChallenge(SMSChallengeInput{Purpose: "phone_bind", Phone: "+8613800138000", IP: "another-ip"})
	assert.ErrorIs(t, err, ErrSMSRateLimit)
}

func TestSMSChallengeActionRollsBackOnBindingConflict(t *testing.T) {
	setupSMSModelTest(t)
	input := SMSChallengeInput{Purpose: "phone_bind", Phone: "13800138000", IP: "127.0.0.1"}
	token, code, challenge, err := ReserveSMSChallenge(input)
	require.NoError(t, err)
	require.NoError(t, MarkSMSDelivered(challenge.Id))
	conflict := errors.New("phone ownership changed")
	_, err = ConsumeSMSChallenge(token, code, input, func(tx *gorm.DB, _ *SMSChallenge) error {
		require.NoError(t, tx.Create(&VerifiedPhone{UserId: 1, Phone: "+8613800138000", Source: "test", VerifiedAt: time.Now()}).Error)
		return conflict
	})
	assert.ErrorIs(t, err, conflict)
	var count int64
	require.NoError(t, DB.Model(&VerifiedPhone{}).Count(&count).Error)
	assert.Zero(t, count)
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	require.NoError(t, err)
}

func TestSMSChallengeRevokedAfterSecurityChange(t *testing.T) {
	setupSMSModelTest(t)
	user := User{Username: "sms-user", Status: common.UserStatusEnabled, AuthVersion: 1, AffCode: "sms-user"}
	require.NoError(t, DB.Create(&user).Error)
	input := SMSChallengeInput{Purpose: "login", Phone: "13800138000", UserId: user.Id, AuthVersion: 1, IP: "127.0.0.1"}
	token, code, challenge, err := ReserveSMSChallenge(input)
	require.NoError(t, err)
	require.NoError(t, MarkSMSDelivered(challenge.Id))
	require.NoError(t, DB.Model(&user).Update("auth_version", 2).Error)
	_, err = ConsumeSMSChallenge(token, code, input, nil)
	assert.ErrorIs(t, err, ErrSMSInvalid)
}

func TestPhoneAvailabilityReservesVerifiedClaimsAndAllMainlandAliases(t *testing.T) {
	setupSMSModelTest(t)
	owner := User{Username: "phone-owner", Status: common.UserStatusEnabled, AuthVersion: 1, AffCode: "phone-owner"}
	require.NoError(t, DB.Create(&owner).Error)
	require.NoError(t, DB.Create(&VerifiedPhone{UserId: owner.Id, Phone: "+8613800138000", Source: "aliyun_sms", VerifiedAt: time.Now()}).Error)
	for _, phone := range []string{"13800138000", "+8613800138000", "8613800138000", "008613800138000"} {
		available, err := IsPhoneAvailable(phone, 0)
		require.NoError(t, err)
		assert.False(t, available, phone)
		available, err = IsPhoneAvailable(phone, owner.Id)
		require.NoError(t, err)
		assert.True(t, available, phone)
	}
	legacy := User{Username: "legacy-phone-owner", Phone: "008613900139000", Status: common.UserStatusEnabled, AffCode: "legacy-phone-owner"}
	require.NoError(t, DB.Create(&legacy).Error)
	err := DB.Transaction(func(tx *gorm.DB) error { return ClaimVerifiedPhoneWithTx(tx, "+8613900139000", owner.Id, "aliyun_sms") })
	assert.ErrorIs(t, err, ErrVerifiedPhoneAlreadyClaimed)
}
