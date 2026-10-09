package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func frozenBillingExportFixture(t *testing.T) (*model.BillingStatement, *BillingSnapshot) {
	t.Helper()
	start, end, err := model.BillingMonthBounds("2026-09")
	require.NoError(t, err)
	snapshot, err := buildBillingSnapshot(&model.BillingAccount{UserID: 42, CompanyTitle: "示例科技有限公司（演示）", TaxID: "DEMO-TAX", AccountingStartAt: start + 27*86400}, "2026-09", []model.BillingHour{{Hour: start + 28*86400, Charge: 500000, Refund: 125000, Count: 2}}, BillingCurrency{Code: "CNY", Symbol: "¥", Rate: "7", QuotaPerUnit: "500000"})
	require.NoError(t, err)
	require.NoError(t, attachBillingModelRows(snapshot, []model.BillingModelTotal{{ModelName: "enterprise-context-analysis-pro-2026-09", Charge: 500000, Refund: 125000, Count: 2, ChargeCount: 1, RefundCount: 1, ActiveDays: 1, FirstPosted: start + 28*86400, LastPosted: start + 28*86400 + 1}}))
	snapshot.PDFTemplateVersion, snapshot.PDFLogoPNG, snapshot.Issuer = 9, billingPlatformMarkV2, "new-api 演示平台"
	snapshot.Username, snapshot.OperatingName = "demo_customer", "演示运营主体"
	snapshot.RateReferenceAt = end + 86400
	snapshot.Models[0].CurrentRates = []BillingGroupRate{{Group: "standard", Ratio: "0.8"}, {Group: "preferred", Ratio: "0.65"}}
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	statement := &model.BillingStatement{ID: "DEMO-EXPORT-202609", UserID: 42, Month: "2026-09", Revision: 2, CreatedAt: end + 86400, StartAt: start + 27*86400, EndAt: end, FromSequence: 1, ToSequence: 2, Snapshot: string(body), SnapshotSHA256: hex.EncodeToString(hash[:])}
	return statement, snapshot
}

func TestBillingExcelPreservesFrozenAmountsRatesAndNoFormulaInjection(t *testing.T) {
	statement, snapshot := frozenBillingExportFixture(t)
	snapshot.Models[0].ModelName = "=HYPERLINK(\"https://example.invalid\")"
	frozen, err := common.Marshal(snapshot)
	require.NoError(t, err)
	hash := sha256.Sum256(frozen)
	statement.Snapshot, statement.SnapshotSHA256 = string(frozen), hex.EncodeToString(hash[:])
	body, err := RenderBillingStatementExcel(statement, snapshot)
	require.NoError(t, err)
	retry, err := RenderBillingStatementExcel(statement, snapshot)
	require.NoError(t, err)
	assert.Equal(t, body, retry, "immutable archives require byte-identical retries")
	book, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer book.Close()
	assert.Equal(t, []string{"总览", "每日汇总", "模型汇总"}, book.GetSheetList())
	for _, cell := range []struct{ sheet, axis, value string }{
		{"总览", "B16", "5.250000"}, {"总览", "B27", statement.SnapshotSHA256},
		{"每日汇总", "D30", "5.250000"}, {"每日汇总", "F2", "正式记账起点之前"},
		{"模型汇总", "H2", "5.250000"}, {"模型汇总", "I2", "65.00–80.00%"}, {"模型汇总", "J2", "未记录"},
		{"模型汇总", "L2", "standard: 80.00%; preferred: 65.00%"},
		{"模型汇总", "A2", snapshot.Models[0].ModelName},
	} {
		value, err := book.GetCellValue(cell.sheet, cell.axis)
		require.NoError(t, err)
		assert.Equal(t, cell.value, value, cell.sheet+"!"+cell.axis)
	}
	formula, err := book.GetCellFormula("模型汇总", "A2")
	require.NoError(t, err)
	assert.Empty(t, formula, "untrusted model names must remain plain text")
	assert.Equal(t, "1234567890.123456", billingExcelAmount("1234567890.123456"), "large amounts must retain exact digits as text")
	assert.Equal(t, 0.000001, billingExcelAmount("0.000001"))
}

func TestBillingArchiveV9BindsExcelAndPDFAndRetriesUnchanged(t *testing.T) {
	truncate(t)
	statement, snapshot := frozenBillingExportFixture(t)
	posted := snapshot.Models[0].FirstPosted
	entries := []model.BillingEntry{
		{EventKey: "export-charge", UserID: 42, Sequence: 1, PostedAt: posted, Kind: "usage", Quota: 500000, ModelName: snapshot.Models[0].ModelName},
		{EventKey: "export-refund", UserID: 42, Sequence: 2, PostedAt: posted + 1, Kind: "refund", Quota: -125000, ModelName: snapshot.Models[0].ModelName},
	}
	require.NoError(t, model.DB.Create(&entries).Error)
	store := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(context.Background(), statement, store))
	require.Len(t, store.files, 5)
	var manifest BillingManifest
	require.NoError(t, common.Unmarshal(store.files["manifest"], &manifest))
	excelHash := sha256.Sum256(store.files["xlsx"])
	assert.Equal(t, hex.EncodeToString(excelHash[:]), statement.ExcelSHA256)
	assert.Equal(t, statement.ExcelSHA256, manifest.ExcelSHA256)
	assert.Equal(t, statement.PDFSHA256, manifest.PDFSHA256)
	assert.Equal(t, statement.SnapshotSHA256, manifest.SnapshotSHA256)
	frozenPDF, frozenExcel, frozenManifest := store.files["pdf"], store.files["xlsx"], store.files["manifest"]
	require.NoError(t, BuildBillingArchive(context.Background(), statement, store))
	assert.Equal(t, frozenPDF, store.files["pdf"])
	assert.Equal(t, frozenExcel, store.files["xlsx"])
	assert.Equal(t, frozenManifest, store.files["manifest"])
	require.NoError(t, model.DB.Create(&model.BillingEntry{EventKey: "late-entry", UserID: 42, Sequence: 3, PostedAt: posted + 2, Kind: "usage", Quota: 100000, ModelName: "late-model"}).Error)
	require.NoError(t, BuildBillingArchive(context.Background(), statement, store))
	assert.Equal(t, frozenPDF, store.files["pdf"], "later ledger entries must not enter the frozen source")
	if output := os.Getenv("BILLING_EXPORT_TEST_OUTPUT"); output != "" {
		require.NoError(t, os.MkdirAll(output, 0700))
		for _, kind := range []string{"pdf", "xlsx", "manifest"} {
			require.NoError(t, os.WriteFile(filepath.Join(output, BillingArtifactFilename(statement, snapshot.Username, kind, 0, snapshot.CompanyTitle)), store.files[kind], 0600))
		}
	}
}

func TestBillingPDFRendersFrozenOperatingLogo(t *testing.T) {
	statement, snapshot := frozenBillingExportFixture(t)
	statement.ExcelSHA256 = strings.Repeat("b", 64)
	withoutLogo, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	// Use separate raster bytes: image deduplication must not hide the operator.
	transport := configureBillingBranding(t)
	common.OperatingEntityLogo = "https://platform.example/operator.png"
	branding, err := captureBillingDocumentBranding(context.Background())
	require.NoError(t, err)
	require.Len(t, transport.requests, 1)
	snapshot.PDFOperatingLogoPNG = branding.OperatingLogoPNG
	withLogo, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Greater(t, strings.Count(string(withLogo), "/Subtype /Image"), strings.Count(string(withoutLogo), "/Subtype /Image"))
	if output := os.Getenv("BILLING_EXPORT_TEST_OUTPUT"); output != "" {
		require.NoError(t, os.WriteFile(filepath.Join(output, "operator-preview.pdf"), withLogo, 0600))
	}
}

func TestBillingFilenamePrefersFrozenCompanyAndSanitizesBothFormats(t *testing.T) {
	statement := &model.BillingStatement{UserID: 42, Month: "2026-09", Revision: 2}
	for _, kind := range []string{"pdf", "xlsx"} {
		assert.Equal(t, "示例_企业_2026-09_月度对账单_V02."+kind, BillingArtifactFilename(statement, "username", kind, 0, "示例/企业\r\n"))
		assert.Equal(t, "username_2026-09_月度对账单_V02."+kind, BillingArtifactFilename(statement, "username", kind, 0, "  "))
	}
}

func TestBillingConfirmationPreservesFrozenOriginalAndCorporateIdentity(t *testing.T) {
	for _, test := range []struct{ name, company, taxID string }{
		{"standard", "上海示例科技有限公司", "DEMO-TAX"},
		{"maximum identity", strings.Repeat("企", 200), strings.Repeat("X", 64)},
	} {
		for _, version := range []int{16, 17} {
			t.Run(fmt.Sprintf("%s/v%d", test.name, version), func(t *testing.T) {
				statement, snapshot := frozenBillingExportFixture(t)
				snapshot.PDFTemplateVersion = version
				snapshot.CompanyTitle, snapshot.TaxID = test.company, test.taxID
				frozen, err := common.Marshal(snapshot)
				require.NoError(t, err)
				snapshotHash := sha256.Sum256(frozen)
				statement.Snapshot, statement.SnapshotSHA256 = string(frozen), hex.EncodeToString(snapshotHash[:])
				workbook, err := RenderBillingStatementExcel(statement, snapshot)
				require.NoError(t, err)
				excelHash := sha256.Sum256(workbook)
				statement.ExcelSHA256 = hex.EncodeToString(excelHash[:])
				original, err := RenderBillingStatementPDF(statement, snapshot, false)
				require.NoError(t, err, "accepted corporate identities and complete fingerprints must fit")
				pdfHash := sha256.Sum256(original)
				statement.PDFSHA256 = hex.EncodeToString(pdfHash[:])
				manifest, err := common.Marshal(BillingManifest{StatementID: statement.ID, PDFSHA256: statement.PDFSHA256, ExcelSHA256: statement.ExcelSHA256, SnapshotSHA256: statement.SnapshotSHA256})
				require.NoError(t, err)
				manifestHash := sha256.Sum256(manifest)
				statement.ManifestSHA256 = hex.EncodeToString(manifestHash[:])
				premature, err := RenderBillingStatementPDF(statement, snapshot, true)
				require.ErrorContains(t, err, "statement is not confirmed", "archived evidence alone must never produce confirmation details")
				assert.Empty(t, premature)
				statement.ConfirmedAt = statement.CreatedAt + 3600
				confirmed, err := RenderBillingStatementPDF(statement, snapshot, true)
				require.NoError(t, err, "the confirmation must fit beside the corporate identity")
				expectedPages := strings.Count(string(original), "\n  /Type /Page\n")
				assert.Equal(t, expectedPages, strings.Count(string(confirmed), "\n  /Type /Page\n"), "confirmation appears on the cover without a separate receipt page")
				assert.NotEqual(t, original, confirmed, "the confirmed artifact must include the confirmation")
				retry, err := RenderBillingStatementPDF(statement, snapshot, false)
				require.NoError(t, err)
				assert.Equal(t, original, retry, "confirmation must not change the original PDF or its fingerprint")
				after, err := common.Marshal(snapshot)
				require.NoError(t, err)
				assert.Equal(t, frozen, after)
				require.NoError(t, statement.VerifySnapshot())
			})
		}
	}
}
