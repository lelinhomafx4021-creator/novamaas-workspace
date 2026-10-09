package service

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingPDFV18PreservesPaletteAndStylesCustomerInformation(t *testing.T) {
	for _, receipt := range []bool{false, true} {
		t.Run(fmt.Sprintf("receipt=%t", receipt), func(t *testing.T) {
			statement, snapshot := frozenBillingExportFixture(t)
			snapshot.PDFTemplateVersion = 18
			snapshot.PDFFooter = "示例对账服务"
			snapshot.DisplayName = "演示客户账户"
			statement.ExcelSHA256 = strings.Repeat("b", 64)
			if receipt {
				statement.ConfirmedAt = statement.CreatedAt + 3600
				statement.PDFSHA256, statement.ManifestSHA256 = strings.Repeat("c", 64), strings.Repeat("d", 64)
			}
			body, err := RenderBillingStatementPDF(statement, snapshot, receipt)
			require.NoError(t, err)
			legacySnapshot := *snapshot
			legacySnapshot.PDFTemplateVersion = 17
			legacyBody, err := RenderBillingStatementPDF(statement, &legacySnapshot, receipt)
			require.NoError(t, err)

			// Inspect the rendered text operators, including headers, amount cells
			// and the confirmation stamp, rather than the template's color calls.
			pageStreams := regexp.MustCompile(`(?s)/Filter/FlateDecode/Length \d+\n>>\nstream\n(.*?)\nendstream`)
			streams := pageStreams.FindAllSubmatch(body, -1)
			legacyStreams := pageStreams.FindAllSubmatch(legacyBody, -1)
			require.Len(t, streams, 3)
			require.Len(t, legacyStreams, len(streams))
			textBlocks := regexp.MustCompile(`(?s)BT\n(.*?)\nET`)
			textColor := regexp.MustCompile(`(?m)^([0-9.]+ [0-9.]+ [0-9.]+) rg$`)
			palette := make(map[string]bool)
			identityColorChecked := false
			coverTextColors := map[string]bool{
				snapshot.Issuer: false,
				"账期起点  " + time.Unix(statement.StartAt, 0).In(billingLocation).Format("2006-01-02 15:04:05"):         false,
				"账期终点  " + time.Unix(statement.EndAt, 0).In(billingLocation).Format("2006-01-02 15:04:05") + " (不含)": false,
				"计价币种  " + snapshot.Currency.Code: false,
				snapshot.CompanyTitle:             false,
				fmt.Sprintf("客户账户  %s (#%d)", snapshot.Username, statement.UserID): false,
				"账户名称  " + snapshot.DisplayName:                                    false,
				"纳税人识别号  " + snapshot.TaxID:                                        false,
				"客户确认（盖章）：":                                                        false,
				fmt.Sprintf("本期明细记录 %d 条  ·  金额统一保留两位小数。", snapshot.Total.Count):   false,
			}
			for page, stream := range streams {
				var contents [2][]byte
				for index, compressed := range [][]byte{stream[1], legacyStreams[page][1]} {
					reader, err := zlib.NewReader(bytes.NewReader(compressed))
					require.NoError(t, err)
					contents[index], err = io.ReadAll(reader)
					require.NoError(t, err)
					require.NoError(t, reader.Close())
				}
				content, legacyContent := contents[0], contents[1]
				assert.Equal(t, textBlocks.ReplaceAll(legacyContent, nil), textBlocks.ReplaceAll(content, nil), "all backgrounds and decorations must retain the original palette on page %d", page+1)
				blocks := textBlocks.FindAllSubmatch(content, -1)
				legacyBlocks := textBlocks.FindAllSubmatch(legacyContent, -1)
				require.NotEmpty(t, blocks)
				require.Len(t, blocks, len(legacyBlocks))
				for index, block := range blocks {
					colors := textColor.FindAllSubmatch(block[1], -1)
					legacyColors := textColor.FindAllSubmatch(legacyBlocks[index][1], -1)
					require.Len(t, colors, 1, "every text block must explicitly set its color on page %d", page+1)
					require.Len(t, legacyColors, 1)
					wantColor := string(legacyColors[0][1])
					text := strings.TrimSpace(billingPDFDecodedText(t, body, block[1]))
					// The issuer also appears in the deep-blue letterhead, which
					// keeps its original color; only the cover body is restyled.
					if _, ok := coverTextColors[text]; page == 0 && ok && string(legacyColors[0][1]) == "0.110 0.169 0.251" {
						wantColor = "0.349 0.412 0.498"
						coverTextColors[text] = true
					}
					if page == 1 && text == fmt.Sprintf("%s  ·  版本 %02d  ·  Asia/Shanghai", snapshot.CompanyTitle, statement.Revision) {
						wantColor = "0.349 0.412 0.498"
						identityColorChecked = true
					}
					assert.Equal(t, wantColor, string(colors[0][1]), "customer information and the cover summary use the notes color; other text retains the original palette on page %d", page+1)
					assert.NotEqual(t, "0.000 0.000 0.000", string(colors[0][1]), "the original palette does not use pure black")
					palette[string(colors[0][1])] = true
				}
			}
			for text, checked := range coverTextColors {
				assert.True(t, checked, "cover text must share the notes' blue-gray color: %s", text)
			}
			require.True(t, identityColorChecked, "the daily customer line must share the notes' blue-gray color")
			assert.Contains(t, palette, "0.098 0.200 0.369", "headings and totals retain deep blue")
			assert.Contains(t, palette, "1.000 1.000 1.000", "dark headers and amount cards retain white text")
			assert.Contains(t, palette, "0.863 0.906 0.969", "dark amount cards retain light blue labels")
			assert.Contains(t, palette, "0.349 0.412 0.498", "notes retain blue-gray text")
			if receipt {
				assert.Contains(t, palette, "0.118 0.400 0.290", "confirmation retains green text")
			}

			dailyText := billingPDFPageText(t, body, 1)
			assert.Contains(t, dailyText, snapshot.CompanyTitle+"  ·  版本 02")
			assert.NotContains(t, dailyText, "抬头")
			modelText := billingPDFPageText(t, body, 2)
			assert.Contains(t, modelText, snapshot.CompanyTitle+"  ·  逐项列示模型金额、涉及账单日和预扣金额占比")
			assert.NotContains(t, modelText, "抬头")
			assert.Contains(t, billingPDFPageText(t, legacyBody, 2), "抬头 "+snapshot.CompanyTitle, "archived templates retain their original customer label")
			if output := os.Getenv("BILLING_STATEMENT_PREVIEW_DIR"); output != "" {
				require.NoError(t, os.MkdirAll(output, 0700))
				require.NoError(t, os.WriteFile(filepath.Join(output, fmt.Sprintf("statement-v18-receipt-%t.pdf", receipt)), body, 0600))
			}
		})
	}
}

func TestBillingPDFAccountBalancesUseAccountingFormat(t *testing.T) {
	statement, branding := frozenBillingExportFixture(t)
	posted := statement.StartAt
	snapshot, err := buildBillingSnapshot(
		&model.BillingAccount{UserID: statement.UserID, CompanyTitle: branding.CompanyTitle, TaxID: branding.TaxID, AccountingStartAt: posted},
		statement.Month,
		[]model.BillingHour{{Hour: posted, Charge: 1610000, Count: 1}, {Hour: posted + 86400, Charge: 11162230000, Count: 1}},
		BillingCurrency{Code: "CNY", Symbol: "¥", Rate: "1", QuotaPerUnit: "1000000"},
	)
	require.NoError(t, err)
	require.NoError(t, attachBillingModelRows(snapshot, []model.BillingModelTotal{{
		ModelName: "doubao-seedance-2-0", Charge: 11163840000, Count: 2, ChargeCount: 2, ActiveDays: 2, FirstPosted: posted, LastPosted: posted + 86400,
	}}))
	snapshot.PDFLogoPNG, snapshot.Issuer, snapshot.Username = branding.PDFLogoPNG, branding.Issuer, branding.Username
	snapshot.PDFTemplateVersion = 18
	body, err := RenderBillingStatementPDF(statement, snapshot, false)
	require.NoError(t, err)
	dailyText := billingPDFPageText(t, body, 1)
	assert.Contains(t, dailyText, "(1.61)")
	assert.Equal(t, 3, strings.Count(dailyText, "(11,163.84)"), "closing negative balance is carried forward and shown in the total")
	assert.NotContains(t, dailyText, "-1.61")
	assert.NotContains(t, dailyText, "-11,163.84")
	assert.Contains(t, dailyText, "0.00", "zero balances retain two places")
	if output := os.Getenv("BILLING_STATEMENT_PREVIEW_DIR"); output != "" {
		require.NoError(t, os.MkdirAll(output, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(output, "statement-balance-accounting.pdf"), body, 0600))
	}
}

// Decode rendered page text through the embedded Unicode map, without requiring
// a machine-installed PDF reader in the regression suite.
func billingPDFPageText(t *testing.T, body []byte, pageIndex int) string {
	t.Helper()
	streams := regexp.MustCompile(`(?s)/Filter/FlateDecode/Length \d+\n>>\nstream\n(.*?)\nendstream`).FindAllSubmatch(body, -1)
	require.Greater(t, len(streams), pageIndex)
	reader, err := zlib.NewReader(bytes.NewReader(streams[pageIndex][1]))
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return billingPDFDecodedText(t, body, content)
}

func billingPDFDecodedText(t *testing.T, body, content []byte) string {
	t.Helper()
	glyphs := make(map[string]rune)
	for _, entry := range regexp.MustCompile(`(?m)^<([0-9A-F]{4})><[0-9A-F]{4}><([0-9A-F]{4})>$`).FindAllSubmatch(body, -1) {
		character, err := strconv.ParseUint(string(entry[2]), 16, 32)
		require.NoError(t, err)
		glyphs[string(entry[1])] = rune(character)
	}
	require.NotEmpty(t, glyphs)
	var text strings.Builder
	for _, encoded := range regexp.MustCompile(`<([0-9A-F]+)>`).FindAllSubmatch(content, -1) {
		for offset := 0; offset+4 <= len(encoded[1]); offset += 4 {
			text.WriteRune(glyphs[string(encoded[1][offset:offset+4])])
		}
		text.WriteByte('\n')
	}
	return text.String()
}
