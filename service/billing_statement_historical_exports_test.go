package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestBillingHistoricalExportsHaveOneConsumedRateAndNoOriginalPriceColumn(t *testing.T) {
	statement, snapshot := frozenBillingExportFixture(t)
	snapshot.PDFTemplateVersion = 12
	snapshot.Models[0].BillingGroup, snapshot.Models[0].BillingRate = "spe_sd_86", "0.86"
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	statement.Snapshot, statement.SnapshotSHA256 = string(body), hex.EncodeToString(hash[:])
	workbook, err := RenderBillingStatementExcel(statement, snapshot)
	require.NoError(t, err)
	retry, err := RenderBillingStatementExcel(statement, snapshot)
	require.NoError(t, err)
	assert.Equal(t, workbook, retry)
	book, err := excelize.OpenReader(bytes.NewReader(workbook))
	require.NoError(t, err)
	defer book.Close()
	rows, err := book.GetRows("模型汇总")
	require.NoError(t, err)
	assert.NotContains(t, rows[0], "标准原价")
	assert.Equal(t, "计费费率", rows[0][8])
	assert.Equal(t, "86.00%", rows[1][8], "current available groups must never appear as consumed rates")
	assert.Equal(t, "spe_sd_86", rows[1][10])
	assert.Equal(t, "5.250000", rows[1][7])
	excelHash := sha256.Sum256(workbook)
	statement.ExcelSHA256 = hex.EncodeToString(excelHash[:])
	pdf, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	snapshot.Models[0].BillingGroup = "unused-display-group-with-a-name-that-would-wrap-over-several-lines"
	withoutGroup, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, pdf, withoutGroup, "PDF layout and content must not include billing group descriptions")
	snapshot.PDFTemplateVersion = 10
	legacy, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	snapshot.Models[0].BillingGroup = "spe_sd_86"
	legacyWithGroup, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, legacy, legacyWithGroup, "all main-template PDFs omit billing group descriptions")
	if output := os.Getenv("BILLING_HISTORICAL_EXPORT_TEST_DIR"); output != "" {
		require.NoError(t, os.MkdirAll(output, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(output, "historical.xlsx"), workbook, 0600))
		require.NoError(t, os.WriteFile(filepath.Join(output, "historical.pdf"), pdf, 0600))
	}
}

func TestBillingArchiveRetryPreservesFrozenHistoricalRatesAfterLogsAreRemoved(t *testing.T) {
	truncate(t)
	seedUser(t, 42, 10000)
	statement, snapshot := frozenBillingExportFixture(t)
	snapshot.PDFTemplateVersion = 10
	snapshot.Models[0].CurrentRates = nil
	snapshot.Models[0].BillingGroup, snapshot.Models[0].BillingRate = "spe_sd_86", "0.86"
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	statement.Snapshot, statement.SnapshotSHA256 = string(body), hex.EncodeToString(hash[:])
	key := "42:2026-09"
	statement.ActiveKey, statement.Status = &key, model.StatementDraft
	require.NoError(t, model.DB.Create(statement).Error)
	require.NoError(t, model.DB.Create(&model.BillingAccount{UserID: 42, Sequence: 2}).Error)
	posted := snapshot.Models[0].FirstPosted
	require.NoError(t, model.DB.Create(&[]model.BillingEntry{
		{EventKey: "frozen-charge", UserID: 42, Sequence: 1, PostedAt: posted, Kind: "usage", Quota: 500000, RequestID: "removed-log", ModelName: snapshot.Models[0].ModelName},
		{EventKey: "frozen-refund", UserID: 42, Sequence: 2, PostedAt: posted + 1, Kind: "task_adjustment", Quota: -125000, RequestID: "removed-task", ModelName: snapshot.Models[0].ModelName},
	}).Error)
	archive := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(context.Background(), statement, archive))
	originalExcel := append([]byte(nil), archive.files["xlsx"]...)
	require.NoError(t, BuildBillingArchive(context.Background(), statement, archive))
	assert.Equal(t, originalExcel, archive.files["xlsx"])
	book, err := excelize.OpenReader(bytes.NewReader(originalExcel))
	require.NoError(t, err)
	defer book.Close()
	rate, err := book.GetCellValue("模型汇总", "I2")
	require.NoError(t, err)
	assert.Equal(t, "86.00%", rate)

}

func TestBillingArchiveReconcilesSplitRateRowsWithoutDuplicatingModelAmounts(t *testing.T) {
	truncate(t)
	statement, snapshot := frozenBillingExportFixture(t)
	snapshot.PDFTemplateVersion = 10
	snapshot.Total.Count = 3
	for i := range snapshot.Days {
		if snapshot.Days[i].Count > 0 {
			snapshot.Days[i].Count = 3
		}
	}
	posted := snapshot.Models[0].FirstPosted
	name := snapshot.Models[0].ModelName
	require.NoError(t, attachBillingModelRows(snapshot, []model.BillingModelTotal{
		{ModelName: name, BillingGroup: "spe_sd_86", BillingRate: "0.86", Charge: 300000, Refund: 125000, Count: 2, ChargeCount: 1, RefundCount: 1, ActiveDays: 1, FirstPosted: posted, LastPosted: posted + 1},
		{ModelName: name, BillingGroup: "sd", BillingRate: "1", Charge: 200000, Count: 1, ChargeCount: 1, ActiveDays: 1, FirstPosted: posted + 2, LastPosted: posted + 2},
	}))
	statement.ToSequence = 3
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	statement.Snapshot, statement.SnapshotSHA256 = string(body), hex.EncodeToString(hash[:])
	require.NoError(t, model.DB.Create(&[]model.BillingEntry{
		{EventKey: "split-charge-a", UserID: 42, Sequence: 1, PostedAt: posted, Kind: "usage", Quota: 300000, ModelName: name},
		{EventKey: "split-refund", UserID: 42, Sequence: 2, PostedAt: posted + 1, Kind: "refund", Quota: -125000, ModelName: name},
		{EventKey: "split-charge-b", UserID: 42, Sequence: 3, PostedAt: posted + 2, Kind: "usage", Quota: 200000, ModelName: name},
	}).Error)
	store := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(context.Background(), statement, store))
	book, err := excelize.OpenReader(bytes.NewReader(store.files["xlsx"]))
	require.NoError(t, err)
	defer book.Close()
	count, err := book.GetCellValue("总览", "B18")
	require.NoError(t, err)
	assert.Equal(t, "1", count, "split rate rows are still one distinct model")
	rows, err := book.GetRows("模型汇总")
	require.NoError(t, err)
	require.Len(t, rows, 4)
	assert.Equal(t, "100.00%", rows[1][8])
	assert.Equal(t, "sd", rows[1][10])
	assert.Equal(t, "86.00%", rows[2][8])
	assert.Equal(t, "spe_sd_86", rows[2][10])
	assert.Equal(t, "5.250000", rows[3][7])
	assert.Equal(t, snapshot.Total.ChargeQuota, snapshot.ChargeQuota)

	snapshot.Models = append(snapshot.Models, snapshot.Models[0])
	body, err = common.Marshal(snapshot)
	require.NoError(t, err)
	hash = sha256.Sum256(body)
	statement.Snapshot, statement.SnapshotSHA256 = string(body), hex.EncodeToString(hash[:])
	require.ErrorContains(t, BuildBillingArchive(context.Background(), statement, &memoryBillingArchive{files: map[string][]byte{}}), "model rate totals exceed ledger amounts", "duplicate split rows must not inflate a customer's bill")
}
