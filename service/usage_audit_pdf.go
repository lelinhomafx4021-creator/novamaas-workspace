package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/signintech/gopdf"
)

type usageAuditPDFCategory struct {
	label, tokens, platform, list, difference string
}

func usageAuditPDFCategories(summary UsageAuditSummary) []usageAuditPDFCategory {
	rows := []usageAuditPDFCategory{
		{label: "普通输入（不含缓存）", tokens: fmt.Sprint(summary.Tokens.Input)},
		{label: "缓存读取", tokens: fmt.Sprint(summary.Tokens.CacheRead)},
		{label: "缓存写入", tokens: fmt.Sprint(summary.Tokens.CacheWrite)},
		{label: "输出", tokens: fmt.Sprint(summary.Tokens.Output)},
	}
	comparison := summary.Comparison
	if comparison != nil {
		for i, costs := range [][3]string{
			{comparison.Platform.Input, comparison.List.Input, comparison.Difference.Input},
			{comparison.Platform.CacheRead, comparison.List.CacheRead, comparison.Difference.CacheRead},
			{comparison.Platform.CacheWrite, comparison.List.CacheWrite, comparison.Difference.CacheWrite},
			{comparison.Platform.Output, comparison.List.Output, comparison.Difference.Output},
		} {
			rows[i].platform, rows[i].list, rows[i].difference = costs[0], costs[1], costs[2]
		}
		for _, adjustment := range []usageAuditPDFCategory{
			{label: "取整差额", tokens: "-", platform: comparison.Platform.Rounding, list: comparison.List.Rounding, difference: comparison.Difference.Rounding},
			{label: "账务调整", tokens: "-", platform: comparison.Platform.Adjustment, list: comparison.List.Adjustment, difference: comparison.Difference.Adjustment},
		} {
			platform, _ := decimal.NewFromString(adjustment.platform)
			list, _ := decimal.NewFromString(adjustment.list)
			if !platform.IsZero() || !list.IsZero() {
				rows = append(rows, adjustment)
			}
		}
	}
	total := usageAuditPDFCategory{label: "合计", tokens: fmt.Sprint(summary.Tokens.TotalInput + summary.Tokens.Output)}
	if comparison != nil {
		total.platform, total.list, total.difference = comparison.Platform.Total, comparison.List.Total, comparison.Difference.Total
	}
	return append(rows, total)
}

func (doc *billingPDFDocument) usageAuditMoney(x, y, width, size float64, value string) {
	if value == "" {
		doc.right(x, y, width, size, "未核价")
		return
	}
	amount, err := decimal.NewFromString(value)
	if err != nil {
		doc.err = err
		return
	}
	places := int32(6)
	for places < 9 && !amount.Equal(amount.Round(places)) {
		places++
	}
	text, err := billingDisplayAmount(value, places, false)
	if err != nil {
		doc.err = err
		return
	}
	doc.right(x, y, width, size, text)
}

func (doc *billingPDFDocument) usageAuditCategoryTable(summary UsageAuditSummary, y float64) float64 {
	rows := usageAuditPDFCategories(summary)
	if y+24+float64(len(rows))*22 > 767 {
		doc.err = errors.New("usage audit table exceeds page boundary")
		return y
	}
	doc.pdf.SetFillColor(25, 51, 94)
	doc.pdf.RectFromUpperLeftWithStyle(40, y, 515, 24, "F")
	doc.pdf.SetTextColor(255, 255, 255)
	doc.text(50, y+7, 145, 8.3, "Token 分类")
	for i, title := range []string{"Token 数", "平台费用", "官价费用", "费用差额"} {
		doc.right(200+float64(i)*87, y+7, 82, 8.3, title)
	}
	y += 24
	for i, row := range rows {
		if i == len(rows)-1 {
			doc.pdf.SetFillColor(232, 239, 249)
		} else if i%2 == 0 {
			doc.pdf.SetFillColor(245, 247, 251)
		} else {
			doc.pdf.SetFillColor(255, 255, 255)
		}
		doc.pdf.RectFromUpperLeftWithStyle(40, y, 515, 22, "F")
		doc.pdf.SetTextColor(35, 49, 69)
		doc.text(50, y+6, 145, 8.2, row.label)
		doc.right(200, y+6, 82, 8.2, row.tokens)
		for col, value := range []string{row.platform, row.list, row.difference} {
			doc.usageAuditMoney(287+float64(col)*87, y+6, 82, 8.2, value)
		}
		y += 22
	}
	return y
}

// Use the existing statement letterhead, operating logo, CJK font, palette and
// footer. A usage comparison has no customer-confirmation or payment state.
func RenderUsageAuditPDF(report *UsageAuditReport, branding *BillingSnapshot) ([]byte, error) {
	if report == nil || branding == nil || len(report.SHA256) != 64 || len(report.Models) > 200 {
		return nil, errors.New("invalid usage audit PDF; select up to 200 models")
	}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.SetInfo(gopdf.PdfInfo{Title: "Token usage and cost reconciliation", Author: branding.Issuer, Creator: "new-api usage audit", CreationDate: time.Unix(report.GeneratedAt, 0).UTC()})
	missingGlyph := false
	if err := pdf.AddTTFFontDataWithOption("billing", billingPDFFont, gopdf.TtfOption{OnGlyphNotFound: func(r rune) { missingGlyph = true }, OnGlyphNotFoundSubstitute: gopdf.DefaultOnGlyphNotFoundSubstitute}); err != nil {
		return nil, err
	}
	type modelSection struct {
		group UsageAuditGroup
		rates string
	}
	pages := make([][]modelSection, 0)
	page := make([]modelSection, 0)
	used := 0.0
	for _, group := range report.Models {
		var rates []string
		for _, rate := range report.Rates {
			if rate.Model == group.Label {
				rates = append(rates, rate.Group+" × "+rate.Ratio)
			}
		}
		if len(rates) > 4 {
			rates = append(rates[:4], fmt.Sprintf("另有 %d 项，见导出明细", len(rates)-4))
		}
		rateText := "历史计费倍率：" + strings.Join(rates, "；")
		if len(rates) == 0 {
			rateText = "历史计费倍率：未记录"
		}
		if err := pdf.SetFont("billing", "", 11); err != nil {
			return nil, err
		}
		nameLines, err := pdf.SplitText(group.Label, 515)
		if err != nil {
			return nil, err
		}
		if err := pdf.SetFont("billing", "", 8); err != nil {
			return nil, err
		}
		rateLines, err := pdf.SplitText(rateText, 515)
		if err != nil {
			return nil, err
		}
		height := float64(len(nameLines))*11*1.45 + float64(len(rateLines))*8*1.45 + 4 + 3 + 8*1.45 + 8 + 24 + float64(len(usageAuditPDFCategories(group.UsageAuditSummary)))*22 + 23
		if height > 589 {
			return nil, errors.New("usage audit model heading exceeds a page")
		}
		if used+height > 589 && len(page) > 0 {
			pages = append(pages, page)
			page = make([]modelSection, 0)
			used = 0
		}
		page = append(page, modelSection{group: group, rates: rateText})
		used += height
	}
	if len(page) > 0 {
		pages = append(pages, page)
	}
	doc := billingPDFDocument{pdf: pdf}
	doc.headerV3(branding, 1, 1+len(pages))
	pdf.SetTextColor(89, 105, 127)
	doc.text(40, 120, 515, 8.5, "TOKEN USAGE & COST RECONCILIATION")
	pdf.SetTextColor(25, 51, 94)
	doc.text(40, 141, 515, 23, "Token 用量与费用核对单")
	pdf.SetTextColor(89, 105, 127)
	doc.text(40, 181, 515, 8.5, "报告编号  UA-"+report.SHA256[:12])
	doc.text(40, 198, 515, 8.5, "生成时间  "+time.Unix(report.GeneratedAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+"  /  Asia/Shanghai")
	left := doc.text(40, 229, 247, 9, "核对客户  "+branding.CompanyTitle)
	if report.Filter.UserID > 0 {
		left = doc.text(40, left+5, 247, 8.5, fmt.Sprintf("客户账户  %s (#%d)", branding.Username, report.Filter.UserID))
	}
	keyLabel := "全部"
	if report.Filter.TokenID > 0 {
		keyLabel = fmt.Sprintf("#%d", report.Filter.TokenID)
	}
	left = doc.text(40, left+5, 247, 8.5, "API Key  "+keyLabel)
	right := doc.text(308, 229, 247, 8.5, "开始  "+time.Unix(report.Filter.StartAt, 0).In(billingLocation).Format("2006-01-02 15:04:05"))
	right = doc.text(308, right+5, 247, 8.5, "结束  "+time.Unix(report.Filter.EndAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+" (不含)")
	right = doc.text(308, right+5, 247, 8.5, "计价币种  "+report.Currency.Code+"  /  换算率 "+report.Currency.Rate)
	filters := make([]string, 0, 4)
	if report.Filter.Model != "" {
		filters = append(filters, "模型 "+report.Filter.Model)
	}
	if report.Filter.Group != "" {
		filters = append(filters, "分组 "+report.Filter.Group)
	}
	if report.Filter.ChannelID > 0 {
		filters = append(filters, fmt.Sprintf("渠道 #%d", report.Filter.ChannelID))
	}
	if report.Filter.Protocol != "" {
		filters = append(filters, "协议 "+report.Filter.Protocol)
	}
	filterText := "全部模型 / 全部分组 / 全部渠道 / 全部协议"
	if len(filters) > 0 {
		filterText = strings.Join(filters, " / ")
	}
	end := doc.text(40, max(left, right)+10, 515, 8, "筛选范围  "+filterText)
	y := max(310.0, end+17)
	for i, metric := range []struct {
		label string
		count int64
	}{
		{"普通输入（不含缓存）", report.Summary.Tokens.Input},
		{"缓存读取", report.Summary.Tokens.CacheRead},
		{"缓存写入", report.Summary.Tokens.CacheWrite},
		{"输出", report.Summary.Tokens.Output},
	} {
		x := 40 + float64(i)*131
		pdf.SetFillColor(232, 239, 249)
		pdf.RectFromUpperLeftWithStyle(x, y, 122, 50, "F")
		pdf.SetTextColor(83, 102, 129)
		doc.text(x+10, y+8, 102, 8, metric.label)
		pdf.SetTextColor(25, 51, 94)
		doc.right(x+10, y+24, 102, 15, fmt.Sprint(metric.count))
	}
	y += 60
	values := [3]string{}
	if report.Summary.Comparison != nil {
		c := report.Summary.Comparison
		values = [3]string{c.Platform.Total, c.List.Total, c.Difference.Total}
	}
	for i, label := range []string{"平台费用 / PLATFORM", "官价费用 / LIST PRICE", "费用差额 / DIFFERENCE"} {
		x := 40 + float64(i)*176
		pdf.SetFillColor(245, 247, 251)
		pdf.RectFromUpperLeftWithStyle(x, y, 163, 46, "F")
		pdf.SetTextColor(83, 102, 129)
		doc.text(x+12, y+8, 139, 8, label)
		pdf.SetTextColor(25, 51, 94)
		doc.usageAuditMoney(x+12, y+24, 139, 13, values[i])
	}
	y += 58
	pdf.SetTextColor(89, 105, 127)
	y = doc.text(40, y, 515, 8.5, fmt.Sprintf("本期 %d 条记录 / 已核价 %d 条 / 缺少可信用量 %d 条。费用比较仅包含已核价记录。", report.Summary.Count, report.Summary.PricedCount, report.Summary.UnknownUsageCount))
	pdf.SetTextColor(25, 51, 94)
	y = doc.text(40, y+13, 515, 11, "全部模型分类汇总")
	y = doc.usageAuditCategoryTable(report.Summary, y+9)
	pdf.SetTextColor(89, 105, 127)
	notes := []string{
		"平台费用为已核价记录的实际收费，已含历史分组倍率；官价采用当时保存的模型基准价，不乘分组倍率。分类费用按历史单价及倍率拆分，账务调整单列。",
		"费用差额 = 官价费用 - 平台费用，反映两种价格口径的差距，不等同于误扣退款金额。缺失价格凭证的记录不猜价，详见 CSV / JSON 明细。",
	}

	if report.RecalculationBasis == "legacy_openai_cache_write_tokens" {
		notes = append(notes, "重算口径：假定历史上游使用 cache_write_tokens；仅统计无缓存写入请求中的缓存读取，有缓存写入的请求排除。")
	}
	if report.Summary.DuplicateReviewCount > 0 {
		value := report.Summary.InputRecalculation
		notes = append(notes, fmt.Sprintf("缓存读取重算：%d 笔 / %d Token；排除有写入请求 %d 笔。按历史普通输入价：平台 %s %s，基准 %s %s。正常缓存费用保留。", value.RequestCount, value.Tokens, value.ExcludedWriteRequestCount, report.Currency.Symbol, value.PlatformAmount, report.Currency.Symbol, value.ListAmount))
	}

	for _, note := range notes {
		y = doc.text(40, y+10, 515, 8, note)
	}
	y = doc.text(40, y+10, 515, 6.5, "数据指纹  "+report.SHA256)
	if y > 767 {
		return nil, errors.New("usage audit cover exceeds page boundary")
	}
	for index, sections := range pages {
		doc.headerV3(branding, index+2, len(pages)+1)
		pdf.SetTextColor(25, 51, 94)
		doc.text(40, 120, 515, 18, "按模型统计")
		pdf.SetTextColor(89, 105, 127)
		doc.text(40, 151, 515, 8.5, "逐模型分开列示普通输入、缓存读取、缓存写入与输出 / "+report.Currency.Code)
		y = 178
		for _, section := range sections {
			pdf.SetTextColor(25, 51, 94)
			y = doc.text(40, y, 515, 11, section.group.Label)
			pdf.SetTextColor(89, 105, 127)
			y = doc.text(40, y+4, 515, 8, section.rates)
			y = doc.text(40, y+3, 515, 8, fmt.Sprintf("记录数 %d / 已核价 %d", section.group.Count, section.group.PricedCount))
			y = doc.usageAuditCategoryTable(section.group.UsageAuditSummary, y+8) + 23
		}
	}
	if doc.err != nil {
		return nil, doc.err
	}
	if missingGlyph {
		return nil, errors.New("usage audit PDF contains unsupported font glyphs")
	}
	return pdf.GetBytesPdfReturnErr()
}

func CreateUsageAuditPDF(ctx context.Context, input UsageAuditExportInput) ([]byte, error) {
	report, err := GetUsageAuditExportReport(ctx, input)
	if err != nil {
		return nil, err
	}
	branding, err := captureBillingDocumentBranding(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := &BillingSnapshot{Issuer: branding.Issuer, PDFLogoPNG: branding.LogoPNG, OperatingName: branding.OperatingName, PDFOperatingLogoPNG: branding.OperatingLogoPNG, PDFFooter: branding.Footer, CompanyTitle: "全部客户"}
	if input.Filter.UserID > 0 {
		customer, err := model.GetUserById(input.Filter.UserID, false)
		if err != nil {
			return nil, err
		}
		account, err := model.GetBillingAccount(input.Filter.UserID)
		if err != nil {
			return nil, err
		}
		snapshot.Username, snapshot.CompanyTitle = customer.Username, account.CompanyTitle
		if snapshot.CompanyTitle == "" {
			snapshot.CompanyTitle = customer.DisplayName
		}
		if snapshot.CompanyTitle == "" {
			snapshot.CompanyTitle = customer.Username
		}
	}
	return RenderUsageAuditPDF(report, snapshot)
}
