package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func correctedStatementTestSnapshot(t *testing.T) (string, string) {
	t.Helper()
	body, err := common.Marshal(map[string]any{"pdf_template_version": 11, "accounting_basis": BillingSourcePeriodCorrections})
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	return string(body), hex.EncodeToString(hash[:])
}

func TestBillingVoidThenCreateUsesSourceMonthAndRetainsFrozenVersions(t *testing.T) {
	start, end, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	input := seedBillingCorrection(t, start+86400)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", input.UserID).Updates(map[string]any{"company_title": "Customer", "tax_id": "TAX"}).Error)
	require.NoError(t, DB.Create(&BillingHour{UserID: input.UserID, Hour: start + 86400, Charge: 848560, Refund: 424280, Count: 3, FirstSequence: 1, LastSequence: 3}).Error)
	body := `{"pdf_template_version":11}`
	hash := sha256.Sum256([]byte(body))
	key := "901:2020-02"
	source := &BillingStatement{ID: "original-confirmed", UserID: input.UserID, Month: "2020-02", Revision: 1, ActiveKey: &key, Status: StatementConfirmed,
		StartAt: start, EndAt: end, FromSequence: 1, ToSequence: 3, Snapshot: body, SnapshotSHA256: hex.EncodeToString(hash[:]), ConfirmedAt: 100, StorageProfileID: 1}
	require.NoError(t, DB.Create(source).Error)
	batch, err := PreviewBillingCorrection(input, 1, "0.77", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.77", "customer_group", 1, false, "")
	require.NoError(t, err)
	var before User
	require.NoError(t, DB.First(&before, input.UserID).Error)
	// The old bill must still see its original frozen amount.
	originalEntries, err := GetBillingStatementEntries(source, 0, 1000)
	require.NoError(t, err)
	require.Len(t, originalEntries, 3)
	assert.Equal(t, int64(424280), originalEntries[0].StatementAmountQuota())
	_, err = ChangeBillingStatement(source.ID, "void", "", "Correct historical rate", "", 1, true)
	require.NoError(t, err)
	next := &BillingStatement{ID: "corrected-version", UserID: source.UserID, Month: source.Month, CreatedBy: 1}
	err = CreateBillingStatement(next, func(_ *BillingAccount, hours []BillingHour, totals []BillingModelTotal) (string, string, error) {
		require.Len(t, hours, 1)
		assert.Equal(t, int64(989990), hours[0].Charge)
		assert.Equal(t, int64(494995), hours[0].Refund)
		assert.Equal(t, int64(3), hours[0].Count)
		require.Len(t, totals, 1)
		assert.Equal(t, "wan-prime", totals[0].BillingGroup)
		assert.Equal(t, "0.77", totals[0].BillingRate)
		assert.Equal(t, int64(2), totals[0].ChargeCount)
		assert.Equal(t, int64(1), totals[0].RefundCount)
		a, b := correctedStatementTestSnapshot(t)
		return a, b, nil
	}, true)
	require.NoError(t, err)
	assert.Equal(t, int64(6), next.ToSequence)
	assert.Equal(t, 2, next.Revision)
	assert.Zero(t, next.ConfirmedAt)
	old, err := GetBillingStatement(source.ID)
	require.NoError(t, err)
	assert.Equal(t, StatementVoid, old.Status)
	assert.Equal(t, source.Snapshot, old.Snapshot)
	assert.Empty(t, old.SupersededBy)
	assert.Equal(t, source.ConfirmedAt, old.ConfirmedAt)
	page, err := GetBillingStatementEntries(next, 0, 1000)
	require.NoError(t, err)
	require.Len(t, page, 3)
	assert.Equal(t, int64(424280), page[0].Quota, "original evidence remains unchanged")
	assert.Equal(t, int64(494995), page[0].StatementAmountQuota())
	require.Len(t, page[0].StatementCorrections, 1)
	assert.Greater(t, page[0].StatementCorrections[0].PostedAt, end, "wallet posting date is not backdated")
	// A reversal is effective in the same source period, while the already
	// frozen corrected bill stays byte-for-byte reproducible.
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "", "", 1, true, "Reverse correction for verification")
	require.NoError(t, err)
	frozen, err := GetBillingStatementEntries(next, 0, 1000)
	require.NoError(t, err)
	assert.Equal(t, page, frozen)
	require.NoError(t, DB.Model(next).Update("status", StatementDraft).Error)
	_, err = ChangeBillingStatement(next.ID, "void", "", "Include reversal", "", 1, true)
	require.NoError(t, err)
	reversed := &BillingStatement{ID: "reversed-version", UserID: source.UserID, Month: source.Month, CreatedBy: 1}
	err = CreateBillingStatement(reversed, func(_ *BillingAccount, hours []BillingHour, totals []BillingModelTotal) (string, string, error) {
		require.Len(t, hours, 1)
		assert.Equal(t, int64(848560), hours[0].Charge)
		assert.Equal(t, int64(424280), hours[0].Refund)
		require.Len(t, totals, 1)
		assert.Equal(t, "wan", totals[0].BillingGroup)
		assert.Equal(t, "0.66", totals[0].BillingRate)
		a, b := correctedStatementTestSnapshot(t)
		return a, b, nil
	}, true)
	require.NoError(t, err)
	// Current posting-date preview contains neither apply nor reversal money.
	postingStart := common.GetTimestamp() / 3600 * 3600
	state, err := GetBillingPreparationState(context.Background(), input.UserID, postingStart, postingStart+3600)
	require.NoError(t, err)
	assert.Empty(t, state.Hours)
	var after User
	require.NoError(t, DB.First(&after, input.UserID).Error)
	assert.Equal(t, 1000000, after.Quota, "financial revisions never mutate the wallet; reversal alone restores it")
	assert.Equal(t, 1000000-int(batch.NetDelta), before.Quota)
}

func TestBillingSourcePeriodRequiresRemovingLegacyPostingPeriodDuplicateFromCurrentVersion(t *testing.T) {
	id := seedBillingCustomer(t)
	start, end, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", id).Updates(map[string]any{"accounting_start_at": start, "sequence": 2}).Error)
	source := BillingEntry{EventKey: "duplicate-period-source", UserID: id, Sequence: 1, PostedAt: start, Kind: "usage", Quota: 66, ModelName: "wan-prime"}
	require.NoError(t, DB.Create(&source).Error)
	require.NoError(t, DB.Create(&BillingEntry{EventKey: "legacy-posting-correction", UserID: id, Sequence: 2, PostedAt: end, Kind: "rate_correction", Quota: 11, ModelName: source.ModelName, SourceEntryID: source.ID, CorrectionID: "batch", BillingGroup: "wan-prime", BillingRate: "0.77"}).Error)
	require.NoError(t, DB.Create(&[]BillingHour{{UserID: id, Hour: start, Charge: 66, Count: 1, FirstSequence: 1, LastSequence: 1}, {UserID: id, Hour: end, Charge: 11, Count: 1, FirstSequence: 2, LastSequence: 2}}).Error)
	_, marchEnd, err := BillingMonthBounds("2020-03")
	require.NoError(t, err)
	body := `{"pdf_template_version":11}`
	hash := sha256.Sum256([]byte(body))
	key := "901:2020-03"
	posting := &BillingStatement{ID: "legacy-posting-period", UserID: id, Month: "2020-03", Revision: 1, ActiveKey: &key, Status: StatementConfirmed, StartAt: end, EndAt: marchEnd, FromSequence: 2, ToSequence: 2, Snapshot: body, SnapshotSHA256: hex.EncodeToString(hash[:]), ConfirmedAt: 100}
	require.NoError(t, DB.Create(posting).Error)
	statement := &BillingStatement{ID: "new-source-period", UserID: id, Month: "2020-02"}
	builder := func(_ *BillingAccount, hours []BillingHour, totals []BillingModelTotal) (string, string, error) {
		a, b := correctedStatementTestSnapshot(t)
		return a, b, nil
	}
	assert.ErrorContains(t, CreateBillingStatement(statement, builder, true), "2020-03")
	_, err = ChangeBillingStatement(posting.ID, "void", "", "Remove duplicate correction period", "", 1, true)
	require.NoError(t, err)
	updatedPosting := &BillingStatement{ID: "corrected-posting-period", UserID: posting.UserID, Month: posting.Month}
	err = CreateBillingStatement(updatedPosting, func(_ *BillingAccount, hours []BillingHour, totals []BillingModelTotal) (string, string, error) {
		assert.Empty(t, hours)
		assert.Empty(t, totals)
		a, b := correctedStatementTestSnapshot(t)
		return a, b, nil
	}, true)
	require.NoError(t, err)
	assert.Equal(t, 2, updatedPosting.Revision)
	require.NoError(t, CreateBillingStatement(statement, func(_ *BillingAccount, hours []BillingHour, totals []BillingModelTotal) (string, string, error) {
		require.Len(t, hours, 1)
		assert.Equal(t, int64(77), hours[0].Charge)
		require.Len(t, totals, 1)
		assert.Equal(t, "0.77", totals[0].BillingRate)
		a, b := correctedStatementTestSnapshot(t)
		return a, b, nil
	}, true))
}

func TestBillingSourcePeriodLowerRateReducesConsumptionAndRefundWithoutChangingTheirCounts(t *testing.T) {
	start, end, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	input := seedBillingCorrection(t, start+86400)
	batch, err := PreviewBillingCorrection(input, 1, "0.55", "customer_group")
	require.NoError(t, err)
	require.True(t, batch.CanApply)
	_, err = ApplyBillingCorrection(batch.ID, batch.SHA256, "0.55", "customer_group", 1, false, "")
	require.NoError(t, err)
	hours, totals, err := billingCorrectedPeriodTotals(DB, input.UserID, 1, 6, start, end)
	require.NoError(t, err)
	require.Len(t, hours, 1)
	assert.Equal(t, int64(707140), hours[0].Charge)
	assert.Equal(t, int64(353570), hours[0].Refund)
	assert.Equal(t, int64(3), hours[0].Count)
	require.Len(t, totals, 1)
	assert.Equal(t, int64(2), totals[0].ChargeCount)
	assert.Equal(t, int64(1), totals[0].RefundCount)
	assert.Equal(t, "0.55", totals[0].BillingRate)
	var user User
	require.NoError(t, DB.First(&user, input.UserID).Error)
	assert.Equal(t, 1070710, user.Quota)
}
