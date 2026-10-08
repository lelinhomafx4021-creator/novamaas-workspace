package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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

func TestBillingPDFV9RendersOperatingLogoAndRequiresWorkbookFingerprint(t *testing.T) {
	statement, snapshot := frozenBillingExportFixture(t)
	_, err := RenderBillingStatementPDF(statement, snapshot, false)
	assert.ErrorContains(t, err, "Excel fingerprint")
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

func TestBillingRatesUsePermittedModelGroupsAndUserOverride(t *testing.T) {
	truncate(t)
	seedUser(t, 42, 10000)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 42).Update("group", "vip").Error)
	savedGroups := setting.UserUsableGroups2JSONString()
	savedRatios := ratio_setting.GroupRatio2JSONString()
	savedOverrides := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(savedOverrides))
		require.NoError(t, model.DB.Where("channel_id IN ?", []int{8001, 8002, 8003, 8004}).Delete(&model.Ability{}).Error)
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":0.8,"vip":0,"private":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"default":0.65}}`))
	abilities := []model.Ability{
		{Group: "default", Model: "Foo", ChannelId: 8001, Enabled: true},
		{Group: "vip", Model: "Foo", ChannelId: 8002, Enabled: true},
		{Group: "private", Model: "Foo", ChannelId: 8003, Enabled: true},
		{Group: "default", Model: "disabled", ChannelId: 8004, Enabled: false},
	}
	require.NoError(t, model.DB.Create(&abilities).Error)
	rates, err := captureBillingRateReferences(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, []BillingGroupRate{{Group: "default", Ratio: "0.65"}, {Group: "vip", Ratio: "0"}}, rates["Foo"])
	assert.NotContains(t, rates, "disabled")
	assert.Equal(t, "0.00–65.00%", billingRateLabel(BillingModelRow{CurrentRates: rates["Foo"]}))
	assert.Equal(t, "0.00%", billingRateLabel(BillingModelRow{CurrentRates: []BillingGroupRate{{Group: "vip", Ratio: "0"}}}))
}

func TestBillingRegenerationKeepsFinancialIdentityAndRefreshesPresentationOnly(t *testing.T) {
	truncate(t)
	seedUser(t, 42, 10000)
	statement, snapshot := frozenBillingExportFixture(t)
	key := "42:2026-09"
	statement.ActiveKey = &key
	statement.Status = model.StatementConfirmed
	statement.ConfirmedAt = statement.CreatedAt + 100
	statement.PDFSHA256, statement.ManifestSHA256 = "archived-pdf", "archived-manifest"
	require.NoError(t, model.DB.Create(statement).Error)
	require.NoError(t, model.DB.Create(&model.BillingAccount{UserID: 42, CompanyTitle: "Changed company", TaxID: "CHANGED", AccountingStartAt: statement.StartAt + 1, Sequence: 3}).Error)
	posted := snapshot.Models[0].FirstPosted
	entries := []model.BillingEntry{
		{EventKey: "regenerate-charge", UserID: 42, Sequence: 1, PostedAt: posted, Kind: "usage", Quota: 500000, ModelName: snapshot.Models[0].ModelName},
		{EventKey: "regenerate-refund", UserID: 42, Sequence: 2, PostedAt: posted + 1, Kind: "refund", Quota: -125000, ModelName: snapshot.Models[0].ModelName},
		{EventKey: "regenerate-late", UserID: 42, Sequence: 3, PostedAt: posted + 2, Kind: "usage", Quota: 900000, ModelName: "later"},
	}
	require.NoError(t, model.DB.Create(&entries).Error)
	configureBillingBranding(t)
	common.SystemName, common.OperatingEntityName = "new-api Updated Brand", "Updated Operator"
	next, err := RegenerateBillingStatementContext(context.Background(), statement, 1)
	require.NoError(t, err)
	var renewed BillingSnapshot
	require.NoError(t, common.UnmarshalJsonStr(next.Snapshot, &renewed))
	assert.Equal(t, snapshot.Currency, renewed.Currency)
	assert.Equal(t, snapshot.CompanyTitle, renewed.CompanyTitle)
	assert.Equal(t, snapshot.TaxID, renewed.TaxID)
	assert.Equal(t, snapshot.AccountingStartAt, renewed.AccountingStartAt)
	assert.Equal(t, snapshot.Days, renewed.Days)
	assert.Equal(t, snapshot.Total, renewed.Total)
	require.Len(t, renewed.Models, 1)
	assert.Equal(t, snapshot.Models[0].ModelName, renewed.Models[0].ModelName)
	assert.Equal(t, snapshot.Models[0].ChargeQuota, renewed.Models[0].ChargeQuota)
	assert.Equal(t, "new-api Updated Brand", renewed.Issuer)
	assert.Equal(t, "Updated Operator", renewed.OperatingName)
	assert.Equal(t, 9, renewed.PDFTemplateVersion)
	archive := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(context.Background(), next, archive))
	assert.NotEmpty(t, archive.files["xlsx"])
	var manifest BillingManifest
	require.NoError(t, common.Unmarshal(archive.files["manifest"], &manifest))
	assert.Equal(t, statement.ID, manifest.SourceStatementID)
	assert.Equal(t, int64(2), manifest.Rows)
}
