package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestBillingAccountingAmountsPreservePrecisionAndSigns(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"12345.678900", "12,345.678900"},
		{"-1233.567890", "(1,233.567890)"},
		{"0.000000", "-"},
		{"-0.000000", "-"},
		{"0.000001", "0.000001"},
		{"1234567890.123456", "1,234,567,890.123456"},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := billingAccountingNumber(test.input)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
	_, err := billingAccountingNumber("invalid")
	assert.Error(t, err)
	snapshot := &BillingSnapshot{PDFTemplateVersion: 14, Currency: BillingCurrency{Symbol: "¥"}}
	assert.Equal(t, "1,234,567,890.123456", billingStatementExcelAmount("1234567890.123456", snapshot))
	snapshot.PDFTemplateVersion = 12
	assert.Equal(t, "1234567890.123456", billingStatementExcelAmount("1234567890.123456", snapshot), "old precision-preserving text cells remain unchanged")
}

func TestBillingDisplayAmountsRoundToTwoPlacesWithoutNegativeZero(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"12345.678900", "12,345.68"},
		{"-1233.567890", "(1,233.57)"},
		{"0.000000", "0.00"},
		{"-0.004000", "0.00"},
		{"0.004999", "0.00"},
		{"0.005000", "0.01"},
		{"-0.005000", "(0.01)"},
		{"1234567890.123456", "1,234,567,890.12"},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := billingDisplayAmount(test.input, 2, false)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestBillingPostpaidBalancesRoundAfterAccumulatingOriginalAmounts(t *testing.T) {
	days := []BillingRow{{State: "outside_period"}, {Amount: "0.004000"}, {Amount: "0.004000"}, {Amount: "0.000000"}, {Amount: "-0.008000"}}
	balances, err := billingPostpaidDailyBalances(days)
	require.NoError(t, err)
	assert.Equal(t, []string{"-", "0.00", "-0.01", "-0.01", "0.00"}, balances)
	assert.Equal(t, "0.004000", days[1].Amount, "rounding must not change the accounting source")
}

func TestBillingModelDetailsAvoidPrematureContinuation(t *testing.T) {
	for _, test := range []struct {
		name       string
		rows       int
		longNames  bool
		modelPages int
	}{
		{"long model names", 8, true, 1},
		{"two models with multiple rates", 12, false, 1},
		{"genuine continuation", 24, true, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			statement, snapshot := frozenBillingExportFixture(t)
			snapshot, err := buildBillingSnapshot(
				&model.BillingAccount{UserID: statement.UserID, CompanyTitle: snapshot.CompanyTitle, TaxID: snapshot.TaxID, AccountingStartAt: statement.StartAt},
				statement.Month,
				[]model.BillingHour{{Hour: statement.StartAt, Charge: int64(test.rows) * 100000, Count: int64(test.rows)}},
				snapshot.Currency,
			)
			require.NoError(t, err)
			snapshot.PDFTemplateVersion, snapshot.PDFLogoPNG, snapshot.Issuer = 17, billingPlatformMarkV2, "new-api 演示平台"
			snapshot.Username = "pagination_customer"
			statement.ExcelSHA256 = strings.Repeat("b", 64)
			totals := make([]model.BillingModelTotal, 0, test.rows)
			for index := 0; index < test.rows; index++ {
				name := fmt.Sprintf("model-%02d", index%2)
				if test.longNames {
					name = fmt.Sprintf("enterprise-reasoning-video-generation-pro-2026-09-model-%02d", index)
				}
				totals = append(totals, model.BillingModelTotal{
					ModelName: name, BillingGroup: fmt.Sprintf("rate-group-%02d", index), BillingRate: "0.8",
					Charge: 100000, Count: 1, ChargeCount: 1, ActiveDays: 1,
					FirstPosted: statement.StartAt, LastPosted: statement.StartAt,
				})
			}
			require.NoError(t, attachBillingModelRows(snapshot, totals))
			pdf, err := RenderBillingStatementPDF(statement, snapshot, false)
			require.NoError(t, err, "model rows, totals and notes must fit without clipping")
			assert.Equal(t, 2+test.modelPages, strings.Count(string(pdf), "\n  /Type /Page\n"))
		})
	}
}

func TestBillingAccountingExportsFormatEveryAmountWithoutChangingFrozenValues(t *testing.T) {
	statement, branding := frozenBillingExportFixture(t)
	start, _, err := model.BillingMonthBounds("2026-09")
	require.NoError(t, err)
	posted := start + 28*86400
	snapshot, err := buildBillingSnapshot(&model.BillingAccount{UserID: 42, CompanyTitle: branding.CompanyTitle, TaxID: branding.TaxID, AccountingStartAt: start + 27*86400}, "2026-09", []model.BillingHour{
		{Hour: posted, Charge: 12345678900, Count: 1},
		{Hour: posted + 86400, Charge: 1000000, Refund: 13580246790, Count: 2},
	}, BillingCurrency{Code: "CNY", Symbol: "¥", Rate: "1", QuotaPerUnit: "1000000"})
	require.NoError(t, err)
	require.NoError(t, attachBillingModelRows(snapshot, []model.BillingModelTotal{
		{ModelName: "example-positive", BillingGroup: "wan-prime", BillingRate: "0.77", Charge: 12345678900, Count: 1, ChargeCount: 1, ActiveDays: 1, FirstPosted: posted, LastPosted: posted},
		{ModelName: "example-negative", BillingGroup: "wan-prime", BillingRate: "0.77", Charge: 1000000, Refund: 13580246790, Count: 2, ChargeCount: 1, RefundCount: 1, ActiveDays: 1, FirstPosted: posted + 86400, LastPosted: posted + 86400},
	}))
	snapshot.PDFTemplateVersion = 14
	snapshot.Issuer, snapshot.PDFLogoPNG, snapshot.OperatingName, snapshot.Username = branding.Issuer, branding.PDFLogoPNG, branding.OperatingName, branding.Username
	frozen, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(frozen)
	statement.Snapshot, statement.SnapshotSHA256 = string(frozen), hex.EncodeToString(hash[:])
	body, err := RenderBillingStatementExcel(statement, snapshot)
	require.NoError(t, err)
	retry, err := RenderBillingStatementExcel(statement, snapshot)
	require.NoError(t, err)
	assert.Equal(t, body, retry)
	book, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer book.Close()
	for _, table := range []struct {
		sheet string
		cells []string
	}{
		{"总览", []string{"B14", "B15", "B16", "B19"}},
		{"每日汇总", nil},
		{"模型汇总", nil},
	} {
		cells := table.cells
		if table.sheet == "每日汇总" {
			for row := 2; row <= len(snapshot.Days)+2; row++ {
				for _, col := range []string{"B", "C", "D"} {
					cells = append(cells, fmt.Sprintf("%s%d", col, row))
				}
			}
		}
		if table.sheet == "模型汇总" {
			for row := 2; row <= len(snapshot.Models)+2; row++ {
				for _, col := range []string{"F", "G", "H"} {
					cells = append(cells, fmt.Sprintf("%s%d", col, row))
				}
			}
		}
		for _, cell := range cells {
			styleID, err := book.GetCellStyle(table.sheet, cell)
			require.NoError(t, err)
			style, err := book.GetStyle(styleID)
			require.NoError(t, err)
			require.NotNil(t, style.CustomNumFmt)
			assert.Equal(t, `_(* #,##0.000000_);_(* (#,##0.000000);_(* "-"??????_);_(@_)`, *style.CustomNumFmt, table.sheet+"!"+cell)
		}
	}
	for _, test := range []struct{ sheet, cell, value string }{
		{"总览", "B14", snapshot.Total.Charge}, {"总览", "B15", snapshot.Total.Refund}, {"总览", "B16", snapshot.Total.Amount}, {"总览", "B19", snapshot.RoundingDifference},
		{"每日汇总", "D31", snapshot.Days[29].Amount}, {"模型汇总", "H3", snapshot.Models[1].Amount},
	} {
		raw, err := book.GetCellValue(test.sheet, test.cell, excelize.Options{RawCellValue: true})
		require.NoError(t, err)
		got, err := decimal.NewFromString(raw)
		require.NoError(t, err)
		want, err := decimal.NewFromString(test.value)
		require.NoError(t, err)
		assert.True(t, got.Equal(want), test.sheet+"!"+test.cell)
	}
	excelHash := sha256.Sum256(body)
	statement.ExcelSHA256 = hex.EncodeToString(excelHash[:])
	pdf, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	pdfHash := sha256.Sum256(pdf)
	statement.PDFSHA256 = hex.EncodeToString(pdfHash[:])
	statement.ManifestSHA256 = statement.SnapshotSHA256
	statement.ConfirmedAt = statement.CreatedAt + 60
	receipt, err := RenderBillingStatementPDF(statement, snapshot, true)
	require.NoError(t, err)
	require.NoError(t, statement.VerifySnapshot())
	if output := os.Getenv("BILLING_ACCOUNTING_EXPORT_TEST_DIR"); output != "" {
		require.NoError(t, os.MkdirAll(output, 0700))
		for name, data := range map[string][]byte{"accounting.xlsx": body, "accounting.pdf": pdf, "receipt.pdf": receipt} {
			require.NoError(t, os.WriteFile(filepath.Join(output, name), data, 0600))
		}
	}
}

func TestBillingAccountingPDFSupportsLargeExactAmounts(t *testing.T) {
	for _, refund := range []bool{false, true} {
		for _, version := range []int{14, 17} {
			t.Run(fmt.Sprintf("refund=%t/v%d", refund, version), func(t *testing.T) {
				statement, snapshot := frozenBillingExportFixture(t)
				charge, refundQuota := int64(common.MaxWalletQuota), int64(0)
				if refund {
					charge, refundQuota = 0, charge
				}
				var err error
				snapshot.Total, err = billingRow(snapshot.Month, charge, refundQuota, 1, snapshot.Currency)
				require.NoError(t, err)
				row, err := billingRow(snapshot.Days[28].Label, charge, refundQuota, 1, snapshot.Currency)
				require.NoError(t, err)
				snapshot.Days[28] = row
				chargeCount, refundCount := int64(1), int64(0)
				if refund {
					chargeCount, refundCount = 0, 1
				}
				snapshot.ChargeQuota, snapshot.RefundQuota = charge, refundQuota
				require.NoError(t, attachBillingModelRows(snapshot, []model.BillingModelTotal{{ModelName: "large-exact-amount", Charge: charge, Refund: refundQuota, Count: 1, ChargeCount: chargeCount, RefundCount: refundCount, ActiveDays: 1, FirstPosted: statement.StartAt, LastPosted: statement.StartAt}}))
				snapshot.PDFTemplateVersion = version
				frozen, err := common.Marshal(snapshot)
				require.NoError(t, err)
				hash := sha256.Sum256(frozen)
				statement.Snapshot, statement.SnapshotSHA256 = string(frozen), hex.EncodeToString(hash[:])
				workbook, err := RenderBillingStatementExcel(statement, snapshot)
				require.NoError(t, err)
				excelHash := sha256.Sum256(workbook)
				statement.ExcelSHA256 = hex.EncodeToString(excelHash[:])
				original, err := RenderBillingStatementPDF(statement, snapshot, false)
				require.NoError(t, err, "large exact ledger amounts must fit accounting columns without clipping")
				if version >= 17 {
					pdfHash := sha256.Sum256(original)
					statement.PDFSHA256 = hex.EncodeToString(pdfHash[:])
					manifest, err := common.Marshal(BillingManifest{StatementID: statement.ID, PDFSHA256: statement.PDFSHA256, ExcelSHA256: statement.ExcelSHA256, SnapshotSHA256: statement.SnapshotSHA256})
					require.NoError(t, err)
					manifestHash := sha256.Sum256(manifest)
					statement.ManifestSHA256 = hex.EncodeToString(manifestHash[:])
					statement.ConfirmedAt = statement.CreatedAt + 60
					_, err = RenderBillingStatementPDF(statement, snapshot, true)
					require.NoError(t, err, "large positive and negative confirmed amounts must fit beside the stamp area")
				}
			})
		}
	}
}

func TestBillingPostpaidDailyBalancesAccumulateConsumptionAndRefunds(t *testing.T) {
	days := []BillingRow{
		{State: "outside_period"},
		{Amount: "100.000000"},
		{Amount: "200.000000"},
		{Amount: "0.000000"},
		{Amount: "-50.000000"},
		{Amount: "0.000001"},
	}
	balances, err := billingPostpaidDailyBalances(days)
	require.NoError(t, err)
	assert.Equal(t, []string{"-", "-100.00", "-300.00", "-300.00", "-250.00", "-250.00"}, balances)
	assert.Equal(t, "200.000000", days[2].Amount, "the daily source amount must not become a running total")
	_, err = billingPostpaidDailyBalances([]BillingRow{{Amount: "invalid"}})
	assert.Error(t, err)
}

func TestBillingPostpaidPDFPreservesFrozenAmountsAndOmitsDailyRecordCounts(t *testing.T) {
	statement, snapshot := frozenBillingExportFixture(t)
	snapshot.PDFTemplateVersion = 15
	statement.ExcelSHA256 = strings.Repeat("b", 64)
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	statement.Snapshot, statement.SnapshotSHA256 = string(body), hex.EncodeToString(hash[:])
	pdf, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	after, err := common.Marshal(snapshot)
	require.NoError(t, err)
	assert.Equal(t, body, after, "rendering postpaid balances must not change frozen billing amounts")
	snapshot.Days[28].Count++
	retry, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, pdf, retry, "daily record counts are not displayed")
}

func TestBillingPostpaidPDFFitsFullMonthAndLargeBalances(t *testing.T) {
	for _, test := range []struct {
		name           string
		charge, refund int64
	}{
		{"consumption", int64(common.MaxWalletQuota), 0},
		{"refund", 0, int64(common.MaxWalletQuota)},
	} {
		t.Run(test.name, func(t *testing.T) {
			statement, branding := frozenBillingExportFixture(t)
			start, end, err := model.BillingMonthBounds("2026-08")
			require.NoError(t, err)
			snapshot, err := buildBillingSnapshot(&model.BillingAccount{UserID: statement.UserID, CompanyTitle: branding.CompanyTitle, TaxID: branding.TaxID, AccountingStartAt: start}, "2026-08", []model.BillingHour{{Hour: start, Charge: test.charge, Refund: test.refund, Count: 1}}, branding.Currency)
			require.NoError(t, err)
			snapshot.PDFTemplateVersion = 15
			snapshot.Issuer, snapshot.PDFLogoPNG, snapshot.Username = branding.Issuer, branding.PDFLogoPNG, branding.Username
			statement.Month, statement.StartAt, statement.EndAt = snapshot.Month, start, end
			statement.ExcelSHA256 = strings.Repeat("b", 64)
			_, err = RenderBillingStatementPDF(statement, snapshot, false)
			require.NoError(t, err, "31 daily rows, opening row, totals and large signed balances must fit")
		})
	}
}
