package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type billingLogoTransport struct {
	body          []byte
	status        int
	contentLength int64
	redirectURL   string
	requests      []*http.Request
}

func (transport *billingLogoTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests = append(transport.requests, request)
	header := make(http.Header)
	if transport.redirectURL != "" {
		header.Set("Location", transport.redirectURL)
	}
	return &http.Response{StatusCode: transport.status, ContentLength: transport.contentLength, Body: io.NopCloser(bytes.NewReader(transport.body)), Header: header}, nil
}

func configureBillingBranding(t *testing.T) *billingLogoTransport {
	t.Helper()
	savedName, savedLogo, savedFooter, savedAddress, savedClient := common.SystemName, common.Logo, common.Footer, system_setting.ServerAddress, httpClient
	savedOperatingName, savedOperatingLogo := common.OperatingEntityName, common.OperatingEntityLogo
	fetchSetting := system_setting.GetFetchSetting()
	savedFetchSetting := *fetchSetting
	common.SystemName, common.Logo, common.Footer = "示例服务平台", "", ""
	common.OperatingEntityName, common.OperatingEntityLogo = "", ""
	system_setting.ServerAddress = "https://platform.example"
	fetchSetting.EnableSSRFProtection = false
	transport := &billingLogoTransport{status: http.StatusOK, body: billingPlatformMarkV2}
	httpClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		common.SystemName, common.Logo, common.Footer, system_setting.ServerAddress, httpClient = savedName, savedLogo, savedFooter, savedAddress, savedClient
		common.OperatingEntityName, common.OperatingEntityLogo = savedOperatingName, savedOperatingLogo
		*fetchSetting = savedFetchSetting
	})
	return transport
}

func TestBillingBrandingCapturesConfiguredLogoAndFooterWithoutURLSecrets(t *testing.T) {
	transport := configureBillingBranding(t)
	common.Logo = "/company-logo.png?signature=do-not-archive"
	common.Footer = `<p>示例科技有限公司 &amp; 服务平台</p><script>do-not-render</script><div>ICP备案 123</div>`
	logo := image.NewNRGBA(image.Rect(0, 0, 300, 150))
	draw.Draw(logo, logo.Bounds(), image.NewUniform(color.NRGBA{R: 10, G: 40, B: 90, A: 255}), image.Point{}, draw.Src)
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, logo))
	transport.body = encoded.Bytes()

	branding, err := captureBillingDocumentBranding(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "示例服务平台", branding.Issuer)
	assert.Equal(t, "示例科技有限公司 & 服务平台 ICP备案 123", branding.Footer)
	decoded, err := png.Decode(bytes.NewReader(branding.LogoPNG))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 256, 128), decoded.Bounds(), "wide logos retain their aspect ratio")
	assert.Equal(t, color.NRGBA{R: 10, G: 40, B: 90, A: 255}, color.NRGBAModel.Convert(decoded.At(120, 60)))
	require.Len(t, transport.requests, 1)
	assert.Equal(t, "https://platform.example/company-logo.png?signature=do-not-archive", transport.requests[0].URL.String())
	assert.Empty(t, transport.requests[0].Header.Get("Authorization"))
	body, err := common.Marshal(branding)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "signature")
	assert.NotContains(t, string(body), "do-not-render")
}

func TestBillingBrandingUsesDefaultOnlyWhenLogoIsUnconfigured(t *testing.T) {
	transport := configureBillingBranding(t)
	branding, err := captureBillingDocumentBranding(context.Background())
	require.NoError(t, err)
	assert.Empty(t, branding.Footer)
	assert.Empty(t, transport.requests, "the default logo needs no network request")
	_, err = png.Decode(bytes.NewReader(branding.LogoPNG))
	require.NoError(t, err)

	common.Logo = "https://platform.example/broken.png"
	transport.status = http.StatusNotFound
	_, err = captureBillingDocumentBranding(context.Background())
	assert.ErrorContains(t, err, "404", "a configured but broken logo must not silently issue the wrong brand")
}

func TestBillingBrandingCapturesSeparateOperatingEntityLogo(t *testing.T) {
	transport := configureBillingBranding(t)
	common.OperatingEntityName = "示例运营有限公司"
	common.OperatingEntityLogo = "https://platform.example/operator.png?signature=private"
	mark := image.NewNRGBA(image.Rect(0, 0, 300, 300))
	draw.Draw(mark, mark.Bounds(), image.NewUniform(color.NRGBA{R: 255, G: 255, B: 255, A: 255}), image.Point{}, draw.Src)
	draw.Draw(mark, image.Rect(20, 125, 280, 175), image.NewUniform(color.NRGBA{R: 160, G: 35, B: 50, A: 255}), image.Point{}, draw.Src)
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, mark))
	transport.body = encoded.Bytes()

	branding, err := captureBillingDocumentBranding(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "示例运营有限公司", branding.OperatingName)
	assert.NotEqual(t, branding.LogoPNG, branding.OperatingLogoPNG)
	config, err := png.DecodeConfig(bytes.NewReader(branding.OperatingLogoPNG))
	require.NoError(t, err)
	assert.Greater(t, config.Width, config.Height*3, "white canvas margins must not shrink the operator mark")
	reportLogo, err := FetchPDFOperatingLogo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, branding.OperatingLogoPNG, reportLogo, "test reports and statements must use identical logo pixels")
	require.Len(t, transport.requests, 2)
	assert.Equal(t, common.OperatingEntityLogo, transport.requests[0].URL.String())
	assert.Empty(t, transport.requests[0].Header.Get("Authorization"))
	body, err := common.Marshal(branding)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "signature")
}

func TestBillingBrandingRejectsUnsafeAndOversizedImages(t *testing.T) {
	for _, test := range []struct {
		name, logo, wantError string
		body                  []byte
		length                int64
		protection            bool
	}{
		{name: "local files", logo: "file:///etc/passwd", wantError: "HTTP(S)"},
		{name: "URL credentials", logo: "https://user:password@platform.example/logo.png", wantError: "credentials"},
		{name: "private metadata", logo: "http://169.254.169.254/latest/meta-data", protection: true, wantError: "security policy"},
		{name: "oversized response", logo: "https://platform.example/logo.png", length: 2097153, wantError: "2 MiB"},
		{name: "HTML instead of image", logo: "https://platform.example/logo.png", body: []byte("<html>login required</html>"), wantError: "valid PNG"},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := configureBillingBranding(t)
			common.Logo = test.logo
			transport.body, transport.contentLength = test.body, test.length
			if test.protection {
				configureSSRFTestFetchSetting(t)
			}
			_, err := captureBillingDocumentBranding(context.Background())
			assert.ErrorContains(t, err, test.wantError)
			if test.protection {
				assert.Empty(t, transport.requests)
			}
		})
	}
	oversized := image.NewNRGBA(image.Rect(0, 0, 4097, 1))
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, oversized))
	_, err := normalizeBillingLogo(encoded.Bytes())
	assert.ErrorContains(t, err, "dimensions")
}

func TestBillingBrandingAllowsConfiguredFakeIPDomain(t *testing.T) {
	transport := configureBillingBranding(t)
	configureSSRFTestFetchSetting(t)
	resolver := staticSSRFResolver{"brand.example": {{IP: net.ParseIP("198.18.0.9")}}}

	logo, err := fetchBillingDocumentLogo(context.Background(), "https://brand.example/logo.png", nil, "Logo", 24*1024, "", resolver)
	require.NoError(t, err)
	_, err = png.Decode(bytes.NewReader(logo))
	require.NoError(t, err)
	require.Len(t, transport.requests, 1)
	assert.Equal(t, "brand.example", transport.requests[0].URL.Hostname())
}

func TestBillingBrandingFakeIPExceptionDoesNotAllowOtherPrivateTargets(t *testing.T) {
	for _, test := range []struct {
		name, logoURL string
		resolved      []net.IPAddr
	}{
		{name: "literal fake IP", logoURL: "https://198.18.0.9/logo.png"},
		{name: "private metadata", logoURL: "https://brand.example/logo.png", resolved: []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}},
		{name: "mixed fake and private", logoURL: "https://brand.example/logo.png", resolved: []net.IPAddr{{IP: net.ParseIP("198.18.0.9")}, {IP: net.ParseIP("127.0.0.1")}}},
		{name: "plain HTTP", logoURL: "http://brand.example/logo.png", resolved: []net.IPAddr{{IP: net.ParseIP("198.18.0.9")}}},
		{name: "disallowed port", logoURL: "https://brand.example:8443/logo.png", resolved: []net.IPAddr{{IP: net.ParseIP("198.18.0.9")}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := configureBillingBranding(t)
			configureSSRFTestFetchSetting(t)
			resolver := staticSSRFResolver{"brand.example": test.resolved}
			_, err := fetchBillingDocumentLogo(context.Background(), test.logoURL, nil, "Logo", 24*1024, "", resolver)
			assert.ErrorIs(t, err, ErrBillingLogoFetchBlocked)
			assert.Empty(t, transport.requests)
		})
	}
}

func TestBillingBrandingFakeIPExceptionDoesNotFollowRedirects(t *testing.T) {
	transport := configureBillingBranding(t)
	configureSSRFTestFetchSetting(t)
	transport.status = http.StatusFound
	transport.redirectURL = "http://169.254.169.254/latest/meta-data"
	resolver := staticSSRFResolver{"brand.example": {{IP: net.ParseIP("198.18.0.9")}}}

	_, err := fetchBillingDocumentLogo(context.Background(), "https://brand.example/logo.png", nil, "Logo", 24*1024, "", resolver)
	assert.Error(t, err)
	assert.Len(t, transport.requests, 1)
}

func TestBillingFooterPreservesTextAndRejectsExcessiveContent(t *testing.T) {
	for _, test := range []struct{ source, expected string }{
		{"", ""},
		{"公司名称\t\n备案号", "公司名称 备案号"},
		{"<div>公司 <a href=\"https://example.com\">备案号</a><br>经营许可</div><style>hidden</style>", "公司 备案号 经营许可"},
		{"<script>alert(1)</script><img src=\"https://example.com/track\">", ""},
	} {
		text, err := billingFooterText(test.source)
		require.NoError(t, err)
		assert.Equal(t, test.expected, text)
	}
	_, err := billingFooterText(strings.Repeat("企", 301))
	assert.ErrorContains(t, err, "300")
}

func TestBillingBrandingFailureDoesNotCreateADraft(t *testing.T) {
	truncate(t)
	seedUser(t, 94, 1000000)
	start, _, err := model.BillingMonthBounds("2020-02")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.BillingAccount{UserID: 94, AccountingStartAt: start, StartSequence: 1, CompanyTitle: "Test", TaxID: "TEST"}).Error)
	configureBillingBranding(t)
	configureSSRFTestFetchSetting(t)
	common.Logo = "http://169.254.169.254/latest/meta-data"
	_, err = PrepareBillingStatement(94, 1, 1, "2020-02")
	assert.ErrorIs(t, err, ErrBillingDocumentBranding)
	assert.ErrorIs(t, err, ErrBillingLogoFetchBlocked)
	assert.ErrorContains(t, err, "private IP address not allowed")
	var count int64
	require.NoError(t, model.DB.Model(&model.BillingStatement{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestBillingDraftArchivesFrozenBrandingAfterSettingsChange(t *testing.T) {
	truncate(t)
	seedUser(t, 95, 1000000)
	start, _, err := model.BillingMonthBounds("2020-02")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.BillingAccount{UserID: 95, AccountingStartAt: start, StartSequence: 1, CompanyTitle: "Test", TaxID: "TEST"}).Error)
	transport := configureBillingBranding(t)
	common.Logo, common.Footer = "https://platform.example/logo.png?signature=private", "<p>客户服务公司</p><p>备案号 123</p>"
	common.OperatingEntityName, common.OperatingEntityLogo = "客户服务公司", "https://platform.example/operator.png?signature=private"
	statement, err := PrepareBillingStatement(95, 1, 1, "2020-02")
	require.NoError(t, err)
	require.Len(t, transport.requests, 2)
	common.SystemName, common.Logo, common.Footer = "Later brand", "https://platform.example/missing.png", "Later footer"
	common.OperatingEntityName, common.OperatingEntityLogo = "Changed operator", "https://platform.example/missing-operator.png"
	transport.status = http.StatusNotFound
	store := &memoryBillingArchive{files: map[string][]byte{}}
	require.NoError(t, BuildBillingArchive(context.Background(), statement, store))
	var archived BillingSnapshot
	require.NoError(t, common.Unmarshal(store.files["snapshot"], &archived))
	assert.Equal(t, "示例服务平台", archived.Issuer)
	assert.Equal(t, "客户服务公司 备案号 123", archived.PDFFooter)
	assert.NotEmpty(t, archived.PDFLogoPNG)
	assert.Equal(t, "客户服务公司", archived.OperatingName)
	assert.NotEmpty(t, archived.PDFOperatingLogoPNG)
	assert.Equal(t, 9, archived.PDFTemplateVersion)
	assert.NotContains(t, string(store.files["snapshot"]), "signature")
	assert.NotEmpty(t, store.files["pdf"])
	assert.NotEmpty(t, statement.PDFSHA256)
	assert.Len(t, transport.requests, 2, "the archive job must not re-fetch changed logos")
}

func TestBillingPDFV3FreezesBrandingForBothOriginalAndReceipt(t *testing.T) {
	configureBillingBranding(t)
	common.Footer = "示例科技有限公司 | 备案信息与服务许可（演示数据）"
	branding, err := captureBillingDocumentBranding(context.Background())
	require.NoError(t, err)
	// Optional local fixture/output hooks allow visual QA without importing any
	// customer records or writing a real statement to the database or OSS.
	if input := os.Getenv("BILLING_PDF_V3_LOGO_INPUT"); input != "" {
		body, err := os.ReadFile(input)
		require.NoError(t, err)
		branding.LogoPNG, err = normalizeBillingLogo(body)
		require.NoError(t, err)
	}
	if issuer := os.Getenv("BILLING_PDF_V3_ISSUER"); issuer != "" {
		branding.Issuer = issuer
	}
	if footer := os.Getenv("BILLING_PDF_V3_FOOTER"); footer != "" {
		branding.Footer, err = billingFooterText(footer)
		require.NoError(t, err)
	}
	start, end, err := model.BillingMonthBounds("2026-08")
	require.NoError(t, err)
	currency := BillingCurrency{Code: "CNY", Symbol: "¥", Rate: "7", QuotaPerUnit: "500000"}
	snapshot, err := buildBillingSnapshot(&model.BillingAccount{UserID: 4, CompanyTitle: "上海示例科技有限公司（演示数据）", TaxID: "DEMO-NOT-A-REAL-TAX-ID", AccountingStartAt: start}, "2026-08", []model.BillingHour{{Hour: start + 9*3600, Charge: 500000, Count: 1}, {Hour: start + 14*86400, Refund: 125000, Count: 1}, {Hour: start + 19*86400, Charge: 750000, Count: 1}}, currency)
	require.NoError(t, err)
	snapshot.PDFTemplateVersion, snapshot.Issuer, snapshot.PDFLogoPNG, snapshot.PDFFooter = 3, branding.Issuer, branding.LogoPNG, branding.Footer
	snapshot.Username, snapshot.DisplayName = "demo_customer", "示例客户（仅展示版式）"
	body, err := common.Marshal(snapshot)
	require.NoError(t, err)
	require.Less(t, len(body), 60*1024)
	snapshotHash := sha256.Sum256(body)
	statement := &model.BillingStatement{ID: "DEMO-202608-DOCUMENT-PREVIEW", UserID: 4, Month: snapshot.Month, Revision: 1, StartAt: start, EndAt: end, CreatedAt: end + 2*86400, Snapshot: string(body), SnapshotSHA256: hex.EncodeToString(snapshotHash[:])}
	original, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Contains(t, string(original), "/Subtype /Image")
	assert.Contains(t, string(original), "/FontFile2")
	common.SystemName, common.Logo, common.Footer = "Changed later", "https://unreachable.example/logo", "Changed footer"
	pdfHash := sha256.Sum256(original)
	statement.PDFSHA256, statement.ManifestSHA256 = hex.EncodeToString(pdfHash[:]), statement.SnapshotSHA256
	statement.ConfirmedAt = statement.CreatedAt + 7200
	unchanged, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, original, unchanged, "later configuration and confirmation never rewrite the original")
	receipt, err := RenderBillingStatementPDF(statement, snapshot, true)
	require.NoError(t, err)
	retry, err := RenderBillingStatementPDF(statement, snapshot, true)
	require.NoError(t, err)
	assert.Equal(t, receipt, retry, "the receipt uses the same frozen branding without network access")
	assert.NotEqual(t, original, receipt)
	snapshot.PDFTemplateVersion = 4
	snapshot.OperatingName = "示例运营有限公司"
	snapshot.PDFOperatingLogoPNG = branding.LogoPNG
	branded, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.NotEqual(t, original, branded, "the new template includes operating entity identity")
	snapshot.PDFTemplateVersion = 5
	previous, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.NotEqual(t, branded, previous, "the v5 layout uses the wide operating mark")
	snapshot.PDFTemplateVersion = 6
	if input := os.Getenv("BILLING_PDF_OPERATING_LOGO_INPUT"); input != "" {
		snapshot.OperatingName = ""
		body, err := os.ReadFile(input)
		require.NoError(t, err)
		snapshot.PDFOperatingLogoPNG, err = normalizeBillingOperatingLogoWithin(body, 12*1024)
		require.NoError(t, err)
		if output := os.Getenv("BILLING_PDF_OPERATING_LOGO_OUTPUT"); output != "" {
			require.NoError(t, os.WriteFile(output, snapshot.PDFOperatingLogoPNG, 0600))
		}
	}
	if name := os.Getenv("BILLING_PDF_OPERATING_NAME"); name != "" {
		snapshot.OperatingName = name
	}
	betterAligned, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.NotEqual(t, previous, betterAligned, "the balanced, text-free letterhead requires a new template version")
	if output := os.Getenv("BILLING_PDF_V6_TEST_OUTPUT"); output != "" {
		require.NoError(t, os.WriteFile(output, betterAligned, 0600))
	}
	snapshot.PDFTemplateVersion = 5
	snapshot.PDFOperatingLogoPNG = branding.LogoPNG
	snapshot.OperatingName = "示例运营有限公司"
	unchangedV5, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, previous, unchangedV5, "previously frozen v5 documents retain their layout")
	snapshot.PDFTemplateVersion = 4
	unchangedV4, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, branded, unchangedV4, "previously frozen v4 documents retain their layout")
	snapshot.PDFTemplateVersion = 3
	snapshot.OperatingName = ""
	snapshot.PDFOperatingLogoPNG = nil
	legacy, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.Equal(t, original, legacy, "the v3 template must keep its archived output")
	if output := os.Getenv("BILLING_PDF_V3_OUTPUT_DIR"); output != "" {
		require.NoError(t, os.WriteFile(filepath.Join(output, BillingArtifactFilename(statement, snapshot.Username, "pdf", 0)), original, 0600))
		require.NoError(t, os.WriteFile(filepath.Join(output, BillingArtifactFilename(statement, snapshot.Username, "receipt", 0)), receipt, 0600))
	}
	snapshot.PDFFooter = ""
	noFooter, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	assert.NotEqual(t, original, noFooter, "configured footer is part of the actual PDF bytes")
	if output := os.Getenv("BILLING_PDF_V3_NO_FOOTER_OUTPUT"); output != "" {
		require.NoError(t, os.WriteFile(output, noFooter, 0600))
	}
	snapshot.PDFFooter = strings.Repeat("过长页脚", 1000)
	_, err = RenderBillingStatementPDF(statement, snapshot, false)
	assert.ErrorContains(t, err, "Footer exceeds", "never silently clip a legal footer")
}

func TestBillingArtifactFilenameUsesFrozenUsernameMonthAndRevision(t *testing.T) {
	statement := &model.BillingStatement{ID: "internal-uuid", UserID: 42, Month: "2026-08", Revision: 2, CreatedAt: time.Date(2026, 9, 3, 0, 0, 0, 0, billingLocation).Unix(), ConfirmedAt: time.Date(2026, 9, 4, 8, 9, 10, 0, billingLocation).Unix()}
	for _, test := range []struct{ kind, expected string }{
		{"pdf", "demo_customer_2026-08_月度对账单_V02.pdf"},
		{"receipt", "demo_customer_2026-08_对账确认回执_V02.pdf"},
		{"details", "statement-2026-08-r2-details-0.jsonl.gz"},
		{"manifest", "statement-2026-08-r2-manifest-0.json"},
	} {
		name := BillingArtifactFilename(statement, "demo_customer", test.kind, 0)
		assert.Equal(t, test.expected, name)
		header := mime.FormatMediaType("attachment", map[string]string{"filename": name})
		mediaType, params, err := mime.ParseMediaType(header)
		require.NoError(t, err)
		assert.Equal(t, "attachment", mediaType)
		assert.Equal(t, test.expected, params["filename"], "Chinese filename survives standard HTTP header encoding")
		assert.NotContains(t, header, statement.ID)
	}
	assert.Equal(t, "team_finance_2026-08_月度对账单_V02.pdf", BillingArtifactFilename(statement, "team/finance\r\n", "pdf", 0))
	assert.Equal(t, "上海示例客户_2026-08_月度对账单_V02.pdf", BillingArtifactFilename(statement, "上海示例客户", "pdf", 0))
	assert.Equal(t, "user-42_2026-08_月度对账单_V02.pdf", BillingArtifactFilename(statement, "", "pdf", 0))
	statement.Month, statement.Revision = "2026-09", 12
	assert.Equal(t, "demo_customer_2026-09_月度对账单_V12.pdf", BillingArtifactFilename(statement, "demo_customer", "pdf", 0))
}
