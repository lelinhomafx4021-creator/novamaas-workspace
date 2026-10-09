package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestBillingCorrectedMonthArchivesNewAmountsRatesAndEvidenceWithoutAnotherWalletCharge(t *testing.T) {
	truncate(t)
	seedUser(t, 42, 10000)
	configureBillingBranding(t)
	settings := operation_setting.GetGeneralSetting()
	savedSettings, savedRate, savedUnit := *settings, operation_setting.USDExchangeRate, common.QuotaPerUnit
	t.Cleanup(func() {
		*settings = savedSettings
		operation_setting.USDExchangeRate = savedRate
		common.QuotaPerUnit = savedUnit
	})
	settings.QuotaDisplayType, operation_setting.USDExchangeRate, common.QuotaPerUnit = "CNY", 7, 500000
	source, snapshot := frozenBillingExportFixture(t)
	snapshot.PDFTemplateVersion = 11
	snapshot.Models[0].CurrentRates = nil
	snapshot.Models[0].BillingGroup, snapshot.Models[0].BillingRate = "wan", "0.66"
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	source.Snapshot, source.SnapshotSHA256 = string(body), hex.EncodeToString(hash[:])
	key := "42:2026-09"
	source.ActiveKey, source.Status = &key, model.StatementConfirmed
	source.Revision, source.StorageProfileID = 1, 1
	source.ConfirmedAt = source.CreatedAt + 1
	source.PDFSHA256, source.ManifestSHA256, source.ExcelSHA256 = "original-pdf", "original-manifest", "original-excel"
	originalReceipt, err := RenderBillingStatementPDF(source, snapshot, true)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(source).Error)
	require.NoError(t, model.DB.Create(&model.BillingAccount{UserID: 42, AccountingStartAt: snapshot.AccountingStartAt, StartSequence: 1, Sequence: 4, CompanyTitle: snapshot.CompanyTitle, TaxID: snapshot.TaxID}).Error)
	posted := snapshot.Models[0].FirstPosted
	entries := []model.BillingEntry{
		{EventKey: "corrected-source-charge", UserID: 42, Sequence: 1, PostedAt: posted, Kind: "history_usage", Quota: 500000, ModelName: snapshot.Models[0].ModelName, ImportID: "history-source", SourceSHA256: "unchanged-source-digest"},
		{EventKey: "corrected-source-refund", UserID: 42, Sequence: 2, PostedAt: posted + 1, Kind: "history_refund", Quota: -125000, ModelName: snapshot.Models[0].ModelName},
	}
	require.NoError(t, model.DB.Create(&entries).Error)
	correctionPosted := source.EndAt + 86400
	require.NoError(t, model.DB.Create(&[]model.BillingEntry{
		{EventKey: "corrected-delta-charge", UserID: 42, Sequence: 3, PostedAt: correctionPosted, Kind: "rate_correction", Quota: 83333, ModelName: entries[0].ModelName, SourceEntryID: entries[0].ID, CorrectionID: "applied-batch", BillingGroup: "wan-prime", BillingRate: "0.77"},
		{EventKey: "corrected-delta-refund", UserID: 42, Sequence: 4, PostedAt: correctionPosted, Kind: "rate_correction", Quota: -20833, ModelName: entries[1].ModelName, SourceEntryID: entries[1].ID, CorrectionID: "applied-batch", BillingGroup: "wan-prime", BillingRate: "0.77"},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.BillingHour{
		{UserID: 42, Hour: posted / 3600 * 3600, Charge: 500000, Refund: 125000, Count: 2, FirstSequence: 1, LastSequence: 2},
		{UserID: 42, Hour: correctionPosted / 3600 * 3600, Charge: 83333, Refund: 20833, Count: 2, FirstSequence: 3, LastSequence: 4},
	}).Error)
	preview, err := GetBillingMonthPreview(context.Background(), 42, "2026-09", 0)
	require.NoError(t, err)
	require.NotNil(t, preview.Formal)
	assert.Equal(t, int64(437500), preview.Formal.ChargeQuota-preview.Formal.RefundQuota)
	assert.Equal(t, int64(2), preview.Formal.Total.Count)
	_, err = model.ChangeBillingStatement(source.ID, "void", "", "Correct historical rates", "", 1, true)
	require.NoError(t, err)
	next, err := PrepareBillingStatementContext(context.Background(), source.UserID, 1, source.StorageProfileID, source.Month)
	require.NoError(t, err)
	var corrected BillingSnapshot
	require.NoError(t, common.UnmarshalJsonStr(next.Snapshot, &corrected))
	assert.Equal(t, 18, corrected.PDFTemplateVersion)
	assert.Equal(t, snapshot.CompanyTitle, corrected.CompanyTitle)
	assert.Equal(t, snapshot.TaxID, corrected.TaxID)
	assert.Equal(t, snapshot.Currency, corrected.Currency)
	assert.Equal(t, "6.125000", corrected.Total.Amount)
	require.Len(t, corrected.Models, 1)
	assert.Equal(t, "0.77", corrected.Models[0].BillingRate)
	assert.Equal(t, "wan-prime", corrected.Models[0].BillingGroup)
	assert.ErrorIs(t, ValidateBillingStatementSource(context.Background(), source), model.ErrBillingStatementCorrectionsPending)
	require.NoError(t, ValidateBillingStatementSource(context.Background(), next))
	archive := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(context.Background(), next, archive))
	book, err := excelize.OpenReader(bytes.NewReader(archive.files["xlsx"]))
	require.NoError(t, err)
	defer book.Close()
	note, err := book.GetCellValue("总览", "B29")
	require.NoError(t, err)
	assert.Equal(t, billingDiscountAmountNote, note)
	amount, err := book.GetCellValue("模型汇总", "G2", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	amountValue, err := decimal.NewFromString(amount)
	require.NoError(t, err)
	assert.Equal(t, "6.125000", amountValue.StringFixed(6))
	rate, err := book.GetCellValue("模型汇总", "I2")
	require.NoError(t, err)
	assert.Equal(t, "77.00%", rate)
	r, err := gzip.NewReader(bytes.NewReader(archive.files["details"]))
	require.NoError(t, err)
	details, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	assert.Contains(t, string(details), `"original_quota":500000`)
	assert.Contains(t, string(details), `"quota":583333`)
	assert.Contains(t, string(details), `"batch_id":"applied-batch"`)
	assert.Contains(t, string(details), `"source_sha256":"unchanged-source-digest"`)
	assert.NotContains(t, string(details), "balance_after")
	assert.NotContains(t, string(details), "actor_id")
	var manifest BillingManifest
	require.NoError(t, common.Unmarshal(archive.files["manifest"], &manifest))
	assert.Equal(t, next.ExcelSHA256, manifest.ExcelSHA256)
	assert.Equal(t, int64(2), manifest.Rows)
	assert.Equal(t, int64(583333), manifest.ChargeQuota)
	assert.Equal(t, int64(145833), manifest.RefundQuota)
	old, err := model.GetBillingStatement(source.ID)
	require.NoError(t, err)
	assert.Equal(t, source.Snapshot, old.Snapshot)
	assert.Equal(t, source.ConfirmedAt, old.ConfirmedAt)
	assert.Equal(t, model.StatementVoid, old.Status)
	retainedReceipt, err := RenderBillingStatementPDF(old, snapshot, true)
	require.NoError(t, err)
	assert.Equal(t, originalReceipt, retainedReceipt, "voiding preserves the exact confirmation receipt")
	var user model.User
	require.NoError(t, model.DB.First(&user, 42).Error)
	assert.Equal(t, 10000, user.Quota)
	// An unrelated later correction must not enter archive retries.
	require.NoError(t, model.DB.Create(&model.BillingEntry{EventKey: "later-correction", UserID: 42, Sequence: 5, PostedAt: correctionPosted + 1, Kind: "rate_correction", Quota: 100, ModelName: entries[0].ModelName, SourceEntryID: entries[0].ID, CorrectionID: "later-batch", BillingGroup: "another", BillingRate: "0.78"}).Error)
	require.NoError(t, model.DB.Model(&model.BillingAccount{}).Where("user_id = ?", 42).Update("sequence", 5).Error)
	assert.ErrorIs(t, ValidateBillingStatementSource(context.Background(), next), model.ErrBillingStatementCorrectionsPending)
	frozenPDF, frozenExcel := append([]byte(nil), archive.files["pdf"]...), append([]byte(nil), archive.files["xlsx"]...)
	require.NoError(t, BuildBillingArchive(context.Background(), next, archive))
	assert.Equal(t, frozenPDF, archive.files["pdf"])
	assert.Equal(t, frozenExcel, archive.files["xlsx"])
	require.NoError(t, model.DB.Model(next).Update("status", model.StatementDraft).Error)
	_, err = model.ChangeBillingStatement(next.ID, "void", "", "Include latest correction", "", 1, true)
	require.NoError(t, err)
	replacement, err := PrepareBillingStatementContext(context.Background(), source.UserID, 1, source.StorageProfileID, source.Month)
	require.NoError(t, err)
	var refreshed BillingSnapshot
	require.NoError(t, common.UnmarshalJsonStr(replacement.Snapshot, &refreshed))
	assert.Equal(t, "6.126400", refreshed.Total.Amount)
	assert.Equal(t, model.BillingSourcePeriodCorrections, refreshed.AccountingBasis)
	assert.Equal(t, 3, replacement.Revision)
	require.NoError(t, BuildBillingArchive(context.Background(), replacement, &memoryBillingArchive{files: map[string][]byte{}}))
}
