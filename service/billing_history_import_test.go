package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type historyImportArchive struct {
	body []byte
	err  error
}

func (store *historyImportArchive) Put(_ context.Context, _ string, _ string, _ int, _ int, _ string, body []byte, _ int64) (*model.BillingArtifact, error) {
	if store.err != nil {
		return nil, store.err
	}
	store.body = body
	return &model.BillingArtifact{ID: 1}, nil
}

func seedBillingHistory(t *testing.T) int64 {
	t.Helper()
	truncate(t)
	seedUser(t, 96, 8765432)
	start, _, err := model.BillingMonthBounds("2020-04")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.BillingAccount{UserID: 96, AccountingStartAt: start, StartSequence: 1, CompanyTitle: "History Customer", TaxID: "HISTORY", ProfileVersion: 1}).Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{Id: 961, UserId: 96, CreatedAt: start + 3600, Type: model.LogTypeConsume, Quota: 500000, RequestId: "old-charge", Content: "never-archive-private-body", Other: "never-archive-admin-data"},
		{Id: 962, UserId: 96, CreatedAt: start + 2*86400, Type: model.LogTypeRefund, Quota: 125000, RequestId: "old-refund"},
		{Id: 963, UserId: 96, CreatedAt: start + 3*86400, Type: model.LogTypeConsume, Quota: 750000, RequestId: "old-second-charge"},
		{Id: 964, UserId: 96, CreatedAt: start + 3600, Type: model.LogTypeTopup, Quota: 9000000},
		{Id: 965, UserId: 97, CreatedAt: start + 3600, Type: model.LogTypeConsume, Quota: 9000000},
	}).Error)
	return start
}

func TestBillingHistoryImportRequiresReviewedEvidenceAndNeverChargesWallet(t *testing.T) {
	seedBillingHistory(t)
	ctx := context.Background()
	review, err := ReviewBillingHistory(ctx, 96, "2020-04")
	require.NoError(t, err)
	require.True(t, review.Ready)
	assert.Equal(t, 3, review.SourceCount)
	assert.Equal(t, int64(1250000), review.Snapshot.ChargeQuota)
	assert.Equal(t, int64(125000), review.Snapshot.RefundQuota)
	var count int64
	require.NoError(t, model.DB.Model(&model.BillingEntry{}).Count(&count).Error)
	assert.Zero(t, count, "review is not consent to import")
	store := &historyImportArchive{}
	batch, err := ConfirmBillingHistoryImport(ctx, 96, 1, 1, "2020-04", "00000000000000000000000000000096", review.SourceSHA256, "Checked against historical consumption and administrator funding records", "live-admin-session", store)
	require.NoError(t, err)
	assert.Equal(t, int64(3), batch.Records)
	assert.Equal(t, int64(1), batch.FromSequence)
	assert.Equal(t, int64(3), batch.ToSequence)
	var user model.User
	require.NoError(t, model.DB.First(&user, 96).Error)
	assert.Equal(t, 8765432, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)
	var entries []model.BillingEntry
	require.NoError(t, model.DB.Order("sequence asc").Find(&entries).Error)
	require.Len(t, entries, 3)
	assert.Equal(t, int64(-125000), entries[1].Quota)
	for _, entry := range entries {
		assert.Zero(t, entry.WalletDelta)
		assert.Equal(t, batch.ID, entry.ImportID)
		assert.Equal(t, "unchanged_at_import", entry.BalanceBasis)
		assert.NotEmpty(t, entry.SourceSHA256)
	}
	reader, err := gzip.NewReader(bytes.NewReader(store.body))
	require.NoError(t, err)
	source, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Equal(t, review.Source, source)
	assert.NotContains(t, string(source), "never-archive")
	retry, err := ConfirmBillingHistoryImport(ctx, 96, 1, 1, "2020-04", "00000000000000000000000000000096", review.SourceSHA256, "retry", "live-admin-session", store)
	require.NoError(t, err)
	assert.Equal(t, batch.ID, retry.ID)
	require.NoError(t, model.DB.Model(&model.BillingEntry{}).Count(&count).Error)
	assert.Equal(t, int64(3), count, "retry must not import or charge twice")
	statement, err := PrepareBillingStatement(96, 1, 1, "2020-04")
	require.NoError(t, err)
	var snapshot BillingSnapshot
	require.NoError(t, common.UnmarshalJsonStr(statement.Snapshot, &snapshot))
	assert.Equal(t, review.Snapshot.Total, snapshot.Total)
	assert.Equal(t, batch.ID, snapshot.HistoryImportID)
	assert.Equal(t, batch.SourceSHA256, snapshot.HistorySourceSHA256)
	assert.Equal(t, "test_user", snapshot.Username)
	assert.Equal(t, 14, snapshot.PDFTemplateVersion)
	assert.NotEmpty(t, snapshot.PDFLogoPNG, "the archive freezes the platform image rather than a mutable URL")
	archive := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(ctx, statement, archive), "imported rows must reconcile with the independent hourly summary")
	reader, err = gzip.NewReader(bytes.NewReader(archive.files["details"]))
	require.NoError(t, err)
	archivedDetails, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Contains(t, string(archivedDetails), batch.ID)
	assert.Contains(t, string(archivedDetails), entries[0].SourceSHA256)
	assert.NotContains(t, string(archivedDetails), "balance")
}

func TestBillingHistoryRejectsChangedSourceBeforeArchivingOrImporting(t *testing.T) {
	start := seedBillingHistory(t)
	review, err := ReviewBillingHistory(context.Background(), 96, "2020-04")
	require.NoError(t, err)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 96, CreatedAt: start + 7200, Type: model.LogTypeConsume, Quota: 1}).Error)
	store := &historyImportArchive{}
	_, err = ConfirmBillingHistoryImport(context.Background(), 96, 1, 1, "2020-04", "00000000000000000000000000000096", review.SourceSHA256, "checked", "session", store)
	assert.ErrorIs(t, err, model.ErrBillingConflict)
	assert.Empty(t, store.body)
	var entries int64
	require.NoError(t, model.DB.Model(&model.BillingEntry{}).Count(&entries).Error)
	assert.Zero(t, entries)
}

func TestBillingHistoryCloudFailureAndConcurrentDraftLeaveLedgerUnchanged(t *testing.T) {
	seedBillingHistory(t)
	review, err := ReviewBillingHistory(context.Background(), 96, "2020-04")
	require.NoError(t, err)
	_, err = ConfirmBillingHistoryImport(context.Background(), 96, 1, 1, "2020-04", "00000000000000000000000000000096", review.SourceSHA256, "checked", "session", &historyImportArchive{err: errors.New("store offline")})
	assert.ErrorContains(t, err, "store offline")
	require.NoError(t, model.DB.Create(&model.BillingStatement{ID: "concurrent-draft", UserID: 96, Month: "2020-04", Status: model.StatementDraft}).Error)
	batch := &model.BillingHistoryImport{ID: "concurrent-import", UserID: 96, Month: "2020-04", SourceSHA256: review.SourceSHA256, ArtifactID: 1, SessionID: "session", Note: "checked"}
	err = model.ApplyBillingHistoryImport(context.Background(), batch, review.Account, review.Records)
	assert.ErrorIs(t, err, model.ErrBillingHistoryBlocked)
	var entries, imports int64
	require.NoError(t, model.DB.Model(&model.BillingEntry{}).Count(&entries).Error)
	require.NoError(t, model.DB.Model(&model.BillingHistoryImport{}).Count(&imports).Error)
	assert.Zero(t, entries)
	assert.Zero(t, imports)
	blocked, err := ReviewBillingHistory(context.Background(), 96, "2020-04")
	require.NoError(t, err)
	assert.False(t, blocked.Ready)
	assert.Equal(t, "concurrent-draft", blocked.ExistingStatement)
}

func TestBillingDraftRejectsLegacyHistoryButAllowsAnActuallyEmptyMonth(t *testing.T) {
	seedBillingHistory(t)
	_, err := PrepareBillingStatement(96, 1, 1, "2020-04")
	assert.ErrorIs(t, err, ErrBillingHistoricalDataUnreconciled)
	statement, err := PrepareBillingStatement(96, 1, 1, "2020-05")
	require.NoError(t, err)
	assert.Equal(t, model.StatementPreparing, statement.Status)
	assert.NoError(t, ValidateBillingStatementSource(context.Background(), statement))
}

func TestBillingHistoryImportPreservesMidMonthCutoffWithExistingAccountHistory(t *testing.T) {
	truncate(t)
	seedUser(t, 96, 8765432)
	start, end, err := model.BillingMonthBounds("2020-04")
	require.NoError(t, err)
	cutoff := start + 27*86400 + 18*3600 + 1800
	account := model.BillingAccount{UserID: 96, AccountingStartAt: cutoff, StartSequence: 3, Sequence: 2, CompanyTitle: "History Customer", TaxID: "HISTORY", ProfileVersion: 1}
	require.NoError(t, model.DB.Create(&account).Error)
	// Earlier wallet history and another month's statement do not require a
	// coverage change when importing only the configured accounting period.
	require.NoError(t, model.DB.Create(&[]model.BillingEntry{
		{UserID: 96, Sequence: 1, PostedAt: cutoff - 2, EventKey: "prior-funding", Kind: "funding"},
		{UserID: 96, Sequence: 2, PostedAt: cutoff - 1, EventKey: "prior-usage", Kind: "usage", Quota: 999999},
	}).Error)
	require.NoError(t, model.DB.Create(&model.BillingStatement{ID: "other-month", UserID: 96, Month: "2020-05", Status: model.StatementDraft}).Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{Id: 961, UserId: 96, CreatedAt: cutoff - 1, Type: model.LogTypeConsume, Quota: 999999, RequestId: "before-cutoff"},
		{Id: 962, UserId: 96, CreatedAt: cutoff, Type: model.LogTypeConsume, Quota: 500000, RequestId: "at-cutoff"},
		{Id: 963, UserId: 96, CreatedAt: cutoff + 1, Type: model.LogTypeRefund, Quota: 125000, RequestId: "after-cutoff"},
		{Id: 964, UserId: 96, CreatedAt: end, Type: model.LogTypeConsume, Quota: 999999, RequestId: "next-month"},
	}).Error)

	ctx := context.Background()
	review, err := ReviewBillingHistory(ctx, 96, "2020-04")
	require.NoError(t, err)
	require.True(t, review.Ready)
	assert.Equal(t, cutoff, review.OldStartAt)
	assert.Equal(t, cutoff, review.NewStartAt)
	require.Len(t, review.Records, 2)
	assert.Equal(t, int64(500000), review.Snapshot.ChargeQuota)
	assert.Equal(t, int64(125000), review.Snapshot.RefundQuota)
	assert.Equal(t, "outside_period", review.Snapshot.Days[26].State)
	assert.NotContains(t, string(review.Source), "before-cutoff")
	assert.NotContains(t, string(review.Source), "next-month")

	store := &historyImportArchive{}
	batch, err := ConfirmBillingHistoryImport(ctx, 96, 1, 1, "2020-04", "00000000000000000000000000000096", review.SourceSHA256, "Verified records from the configured cutoff", "session", store)
	require.NoError(t, err)
	assert.Equal(t, cutoff, batch.OldStartAt)
	assert.Equal(t, cutoff, batch.NewStartAt)
	assert.Equal(t, int64(2), batch.Records)
	updated, err := model.GetBillingAccount(96)
	require.NoError(t, err)
	assert.Equal(t, cutoff, updated.AccountingStartAt)
	assert.Equal(t, account.StartSequence, updated.StartSequence)
	var user model.User
	require.NoError(t, model.DB.First(&user, 96).Error)
	assert.Equal(t, 8765432, user.Quota)
	statement, err := PrepareBillingStatement(96, 1, 1, "2020-04")
	require.NoError(t, err)
	assert.Equal(t, cutoff, statement.StartAt)
	archive := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(ctx, statement, archive))
}

func TestBillingHistoryImportRejectsRecordsBeforeCutoffAtCommit(t *testing.T) {
	start := seedBillingHistory(t)
	cutoff := start + 1800
	require.NoError(t, model.DB.Model(&model.BillingAccount{}).Where("user_id = ?", 96).Update("accounting_start_at", cutoff).Error)
	review, err := ReviewBillingHistory(context.Background(), 96, "2020-04")
	require.NoError(t, err)
	records := append(review.Records, model.BillingHistoryRecord{ID: 999, UserID: 96, CreatedAt: cutoff - 1, Type: model.LogTypeConsume, Quota: 1})
	batch := &model.BillingHistoryImport{ID: "invalid-cutoff", UserID: 96, Month: "2020-04", SourceSHA256: review.SourceSHA256, ArtifactID: 1, SessionID: "session", Note: "checked"}
	err = model.ApplyBillingHistoryImport(context.Background(), batch, review.Account, records)
	assert.ErrorIs(t, err, model.ErrBillingEvidenceIntegrity)
	var entries int64
	require.NoError(t, model.DB.Model(&model.BillingEntry{}).Count(&entries).Error)
	assert.Zero(t, entries)
	account, err := model.GetBillingAccount(96)
	require.NoError(t, err)
	assert.Equal(t, cutoff, account.AccountingStartAt)
}

func TestBillingHistoryReviewRejectsUnconfiguredOrUncoveredPeriods(t *testing.T) {
	for _, scenario := range []string{"unconfigured", "before-start"} {
		t.Run(scenario, func(t *testing.T) {
			seedBillingHistory(t)
			_, end, err := model.BillingMonthBounds("2020-04")
			require.NoError(t, err)
			cutoff := int64(0)
			if scenario == "before-start" {
				cutoff = end
			}
			require.NoError(t, model.DB.Model(&model.BillingAccount{}).Where("user_id = ?", 96).Update("accounting_start_at", cutoff).Error)
			review, err := ReviewBillingHistory(context.Background(), 96, "2020-04")
			require.NoError(t, err)
			assert.False(t, review.Ready)
			assert.Empty(t, review.Records)
			assert.Equal(t, cutoff, review.NewStartAt)
			store := &historyImportArchive{}
			_, err = ConfirmBillingHistoryImport(context.Background(), 96, 1, 1, "2020-04", "00000000000000000000000000000096", review.SourceSHA256, "checked", "session", store)
			assert.ErrorIs(t, err, model.ErrBillingHistoryBlocked)
			assert.Empty(t, store.body)
		})
	}
}

func TestBillingPreCutoffHistoryAllowsEmptyFormalStatement(t *testing.T) {
	start := seedBillingHistory(t)
	cutoff := start + 27*86400 + 18*3600
	require.NoError(t, model.DB.Model(&model.BillingAccount{}).Where("user_id = ?", 96).Update("accounting_start_at", cutoff).Error)
	ctx := context.Background()
	preview, err := GetBillingMonthPreview(ctx, 96, "2020-04", 0)
	require.NoError(t, err)
	assert.Equal(t, "no_consumption", preview.Readiness.Status)
	require.NotNil(t, preview.Formal)
	assert.Zero(t, preview.Formal.Total.Count)
	assert.Equal(t, int64(3), preview.Reference.Total.Count)
	review, err := ReviewBillingHistory(ctx, 96, "2020-04")
	require.NoError(t, err)
	assert.Empty(t, review.Records)
	assert.False(t, review.Ready, "there are no in-period records to import")
	statement, err := PrepareBillingStatement(96, 1, 1, "2020-04")
	require.NoError(t, err)
	assert.Equal(t, cutoff, statement.StartAt)
	require.NoError(t, ValidateBillingStatementSource(ctx, statement))
	archive := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(ctx, statement, archive))
}

func TestBillingHistoryImportRejectsChangedCutoffAfterReview(t *testing.T) {
	start := seedBillingHistory(t)
	ctx := context.Background()
	review, err := ReviewBillingHistory(ctx, 96, "2020-04")
	require.NoError(t, err)
	cutoff := start + 1800
	require.NoError(t, model.DB.Model(&model.BillingAccount{}).Where("user_id = ?", 96).Update("accounting_start_at", cutoff).Error)
	store := &historyImportArchive{}
	_, err = ConfirmBillingHistoryImport(ctx, 96, 1, 1, "2020-04", "00000000000000000000000000000096", review.SourceSHA256, "checked", "session", store)
	assert.ErrorIs(t, err, model.ErrBillingConflict)
	assert.Empty(t, store.body)
	batch := &model.BillingHistoryImport{ID: "stale-cutoff", UserID: 96, Month: "2020-04", SourceSHA256: review.SourceSHA256, ArtifactID: 1, SessionID: "session", Note: "checked"}
	assert.ErrorIs(t, model.ApplyBillingHistoryImport(ctx, batch, review.Account, review.Records), model.ErrBillingConflict)
	var entries int64
	require.NoError(t, model.DB.Model(&model.BillingEntry{}).Count(&entries).Error)
	assert.Zero(t, entries)
}

func TestBillingHistoryImportRejectsExistingSummaryInPartialStartingHour(t *testing.T) {
	start := seedBillingHistory(t)
	cutoff := start + 1800
	require.NoError(t, model.DB.Model(&model.BillingAccount{}).Where("user_id = ?", 96).Update("accounting_start_at", cutoff).Error)
	require.NoError(t, model.DB.Create(&model.BillingHour{UserID: 96, Hour: start, Charge: 1, Count: 1, FirstSequence: 1, LastSequence: 1}).Error)
	review, err := ReviewBillingHistory(context.Background(), 96, "2020-04")
	require.NoError(t, err)
	assert.False(t, review.Ready)
	assert.Contains(t, review.Checks, BillingCheck{Code: "history_empty_ledger", Passed: false})
	batch := &model.BillingHistoryImport{ID: "mixed-partial-hour", UserID: 96, Month: "2020-04", SourceSHA256: review.SourceSHA256, ArtifactID: 1, SessionID: "session", Note: "checked"}
	assert.ErrorIs(t, model.ApplyBillingHistoryImport(context.Background(), batch, review.Account, review.Records), model.ErrBillingHistoryBlocked)
	var entries int64
	require.NoError(t, model.DB.Model(&model.BillingEntry{}).Count(&entries).Error)
	assert.Zero(t, entries)
}
