package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/signintech/gopdf"
)

// Render letterheads from the frozen branding and identity.
func (doc *billingPDFDocument) headerV3(snapshot *BillingSnapshot, page, pages int) {
	if doc.err != nil {
		return
	}
	doc.pdf.AddPage()
	if len(snapshot.PDFLogoPNG) == 0 {
		doc.err = errors.New("billing document has no frozen platform logo")
		return
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(snapshot.PDFLogoPNG))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		doc.err = errors.New("billing document platform logo is invalid")
		return
	}
	mark, err := gopdf.ImageHolderByBytes(snapshot.PDFLogoPNG)
	if err != nil {
		doc.err = err
		return
	}
	scale := math.Min(38/float64(config.Width), 38/float64(config.Height))
	width, height := float64(config.Width)*scale, float64(config.Height)*scale

	platformTop, titleTop := 42.0, 43.0

	if doc.err = doc.pdf.ImageByHolder(mark, 40+(38-width)/2, platformTop+(38-height)/2, &gopdf.Rect{W: width, H: height}); doc.err != nil {
		return
	}
	doc.pdf.SetTextColor(25, 51, 94)
	platformWidth := 465.0
	if len(snapshot.PDFOperatingLogoPNG) > 0 || snapshot.OperatingName != "" {
		platformWidth = 335
	}
	end := doc.text(90, titleTop, platformWidth, 15, snapshot.Issuer)
	if end > 80 {
		doc.err = errors.New("platform name exceeds PDF letterhead")
		return
	}
	doc.pdf.SetTextColor(90, 106, 129)
	if platformWidth < 465 {
		doc.text(90, end+1, platformWidth, 7.5, "CUSTOMER RECONCILIATION  /  客户服务消费对账")
		if len(snapshot.PDFOperatingLogoPNG) > 0 {
			config, _, err := image.DecodeConfig(bytes.NewReader(snapshot.PDFOperatingLogoPNG))
			if err != nil || config.Width <= 0 || config.Height <= 0 {
				doc.err = errors.New("billing document operating entity logo is invalid")
				return
			}
			operatingMark, err := gopdf.ImageHolderByBytes(snapshot.PDFOperatingLogoPNG)
			if err != nil {
				doc.err = err
				return
			}
			scale := math.Min(125/float64(config.Width), 40/float64(config.Height))
			width, height := float64(config.Width)*scale, float64(config.Height)*scale
			if doc.err = doc.pdf.ImageByHolder(operatingMark, 555-width, 42+(40-height)/2, &gopdf.Rect{W: width, H: height}); doc.err != nil {
				return
			}
		} else {
			doc.right(430, 54, 125, 10, snapshot.OperatingName)
		}
	} else {
		doc.text(90, end+1, 465, 8, "CUSTOMER RECONCILIATION  /  客户服务消费对账")
	}
	doc.pdf.SetStrokeColor(39, 71, 122)
	doc.pdf.SetLineWidth(1.5)
	doc.pdf.Line(40, 100, 555, 100)
	doc.pdf.SetStrokeColor(221, 228, 237)
	doc.pdf.SetLineWidth(0.5)
	doc.pdf.Line(40, 784, 555, 784)
	doc.pdf.SetTextColor(99, 113, 132)
	if snapshot.PDFFooter != "" {
		fitted := false
		for size := 7.5; size >= 6; size -= 0.5 {
			if doc.err = doc.pdf.SetFont("billing", "", size); doc.err != nil {
				return
			}
			lines, err := doc.pdf.SplitText(snapshot.PDFFooter, 445)
			if err != nil {
				doc.err = err
				return
			}
			if 790+float64(len(lines)-1)*size*1.45+size <= 820 {
				doc.text(40, 790, 445, size, snapshot.PDFFooter)
				fitted = true
				break
			}
		}
		if !fitted {
			doc.err = errors.New("billing Footer exceeds available page space; shorten the system Footer")
			return
		}
	}
	doc.right(500, 794, 55, 8, fmt.Sprintf("%02d / %02d", page, pages))
	doc.pdf.SetTextColor(28, 43, 64)
}

func billingModelRowHeight(pdf *gopdf.GoPdf, row BillingModelRow, nameWidth float64) (float64, error) {
	if err := pdf.SetFont("billing", "", 8.2); err != nil {
		return 0, err
	}
	lines, err := pdf.SplitText(billingModelDisplayName(row), nameWidth)
	if err != nil {
		return 0, err
	}
	height := math.Max(20, float64(len(lines))*8.2*1.45+8)
	if row.ActiveDays > 0 {
		height += 13
	}

	height = math.Max(32, height)

	return height, nil
}

func billingModelSummaryPages(pdf *gopdf.GoPdf, rows []BillingModelRow, nameWidth, footerHeight, headingOffset float64) ([][]BillingModelRow, error) {
	if len(rows) == 0 {
		return [][]BillingModelRow{{}}, nil
	}
	pages := make([][]BillingModelRow, 0, 1)
	page := make([]BillingModelRow, 0, 24)
	height := 0.0

	// The first table starts at 279; continuation tables start at 218.
	// Keep totals and reconciliation notes inside the 767-point content limit.
	pageHeight := 767 - 279 - headingOffset - footerHeight

	for _, row := range rows {
		rowHeight, err := billingModelRowHeight(pdf, row, nameWidth)
		if err != nil {
			return nil, err
		}
		if rowHeight > pageHeight {
			return nil, errors.New("billing model name exceeds available page space")
		}
		if height+rowHeight > pageHeight && len(page) > 0 {
			pages = append(pages, page)
			page = make([]BillingModelRow, 0, 24)
			height = 0

			pageHeight = 767 - 218 - headingOffset - footerHeight

		}
		page = append(page, row)
		height += rowHeight
	}
	return append(pages, page), nil
}

func renderBillingStatementPDFV3(statement *model.BillingStatement, snapshot *BillingSnapshot, receipt bool) ([]byte, error) {

	if receipt && (statement.ConfirmedAt <= 0 || statement.PDFSHA256 == "" || statement.ManifestSHA256 == "") {
		return nil, errors.New("statement is not confirmed with archived evidence")
	}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.SetInfo(gopdf.PdfInfo{Title: "Monthly statement of account / " + statement.Month, Author: snapshot.Issuer, Creator: "new-api billing statements", CreationDate: time.Unix(statement.CreatedAt, 0).UTC()})
	missingGlyph := false
	if err := pdf.AddTTFFontDataWithOption("billing", billingPDFFont, gopdf.TtfOption{OnGlyphNotFound: func(r rune) { missingGlyph = true }, OnGlyphNotFoundSubstitute: gopdf.DefaultOnGlyphNotFoundSubstitute}); err != nil {
		return nil, err
	}

	identitySize, identityHeight, modelHeadingOffset := 8.2, 0.0, 0.0

	dailyIdentity := fmt.Sprintf("抬头 %s  ·  版本 %02d  ·  Asia/Shanghai", snapshot.CompanyTitle, statement.Revision)
	modelIdentity := "抬头 " + snapshot.CompanyTitle + "  ·  逐项列示模型金额、涉及账单日和预扣金额占比"
	if snapshot.PDFTemplateVersion >= 18 {
		dailyIdentity = fmt.Sprintf("%s  ·  版本 %02d  ·  Asia/Shanghai", snapshot.CompanyTitle, statement.Revision)
		modelIdentity = snapshot.CompanyTitle + "  ·  逐项列示模型金额、涉及账单日和预扣金额占比"
	}
	// Keep complete corporate titles above the tables, including long titles.
	for ; identitySize >= 6; identitySize -= 0.2 {
		if err := pdf.SetFont("billing", "", identitySize); err != nil {
			return nil, err
		}
		identityHeight = 0
		for _, text := range []string{dailyIdentity, modelIdentity} {
			lines, err := pdf.SplitText(text, 515)
			if err != nil {
				return nil, err
			}
			identityHeight = math.Max(identityHeight, float64(len(lines))*identitySize*1.45)
		}
		if identityHeight <= 27 {
			break
		}
	}
	if identitySize < 6 {
		return nil, errors.New("billing corporate title exceeds detail heading space")
	}
	modelHeadingOffset = math.Max(0, identityHeight-8.2*1.45)

	modelNote := "实际消费金额 = 预扣金额 - 退款金额；金额占比 = 单模型预扣金额 ÷ 本期预扣金额。 同一模型按消费时实际计费组及费率分行列示；未记录表示历史证据缺失。 实际计费组详见随附 Excel。"
	if err := pdf.SetFont("billing", "", 8); err != nil {
		return nil, err
	}
	lines, err := pdf.SplitText(modelNote, 515)
	if err != nil {
		return nil, err
	}
	modelFooterHeight := 37 + float64(len(lines))*8*1.45 + 4 + 8*1.45
	if snapshot.ChargeQuota > 0 {
		modelFooterHeight += 4 + 8*1.45
	}

	modelPages, pageErr := billingModelSummaryPages(pdf, snapshot.Models, 168, modelFooterHeight, modelHeadingOffset)
	if pageErr != nil {
		return nil, pageErr
	}

	pages := 2 + len(modelPages)

	doc := billingPDFDocument{pdf: pdf}
	doc.headerV3(snapshot, 1, pages)
	pdf.SetTextColor(89, 105, 127)
	doc.text(40, 121, 360, 8.5, "STATEMENT OF ACCOUNT")
	pdf.SetTextColor(25, 51, 94)
	doc.text(40, 141, 350, 26, "月度对账单")
	doc.right(410, 144, 145, 22, statement.Month)
	pdf.SetTextColor(88, 104, 126)
	doc.text(40, 185, 515, 8.5, fmt.Sprintf("文件编号  %s   /   版本 %02d", statement.ID, statement.Revision))
	doc.text(40, 202, 515, 8.5, "生成时间  "+time.Unix(statement.CreatedAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+"   /   Asia/Shanghai (UTC+08:00)")
	pdf.SetTextColor(89, 105, 127)
	doc.text(40, 242, 232, 8.5, "服务平台 / SERVICE PROVIDER")
	doc.text(308, 242, 247, 8.5, "对账客户 / BILL TO")
	pdf.SetTextColor(28, 43, 64)
	if snapshot.PDFTemplateVersion >= 18 {
		pdf.SetTextColor(89, 105, 127)
	}
	left := doc.text(40, 264, 232, 12, snapshot.Issuer)
	left = doc.text(40, left+14, 232, 9, "账期起点  "+time.Unix(statement.StartAt, 0).In(billingLocation).Format("2006-01-02 15:04:05"))
	left = doc.text(40, left+5, 232, 9, "账期终点  "+time.Unix(statement.EndAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+" (不含)")

	currencyLabel := "计价币种  " + snapshot.Currency.Code

	left = doc.text(40, left+5, 232, 9, currencyLabel)

	companySize := 9.0

	right := doc.text(308, 264, 247, companySize, snapshot.CompanyTitle)
	right = doc.text(308, right+12, 247, 9, fmt.Sprintf("客户账户  %s (#%d)", snapshot.Username, statement.UserID))
	if snapshot.DisplayName != "" && snapshot.DisplayName != snapshot.CompanyTitle {
		right = doc.text(308, right+4, 247, 9, "账户名称  "+snapshot.DisplayName)
	}
	right = doc.text(308, right+4, 247, 9, "纳税人识别号  "+snapshot.TaxID)

	sealTop := doc.text(308, right+12, 247, 9, "客户确认（盖章）：") + 6
	// Both copies reserve the same space for a customer stamp and the
	// confirmation recorded by the platform.
	if receipt {
		confirmedAmount, err := billingDisplayAmount(snapshot.Total.Amount, 2, false)
		if err != nil {
			return nil, err
		}
		pdf.SetFillColor(237, 246, 241)
		pdf.RectFromUpperLeftWithStyle(410, sealTop, 125, 56, "F")
		pdf.SetTextColor(30, 102, 74)
		doc.text(420, sealTop+6, 105, 9.5, "客户账户已执行确认")
		doc.text(420, sealTop+20, 105, 7, "确认时间（北京时间）")
		doc.text(420, sealTop+31, 105, 7.5, time.Unix(statement.ConfirmedAt, 0).In(billingLocation).Format("2006-01-02 15:04:05"))
		doc.right(416, sealTop+43, 113, 8, "确认消费金额："+confirmedAmount+" "+snapshot.Currency.Code)
	}
	right = sealTop + 76

	y := max(left, right) + 27
	y -= 5

	coverAmounts := []struct{ label, value string }{
		{"预扣金额 / PRECHARGES", snapshot.Total.Charge},
		{"实际消费金额 / ACTUAL USAGE", snapshot.Total.Amount},
		{"退款金额 / REFUNDS", snapshot.Total.Refund},
	}
	actualConsumptionCard := 1

	for i, item := range coverAmounts {
		x := 40 + float64(i)*176
		pdf.SetFillColor(241, 245, 251)
		if i == actualConsumptionCard {
			pdf.SetFillColor(25, 51, 94)
		}
		pdf.RectFromUpperLeftWithStyle(x, y, 163, 79, "F")
		pdf.SetTextColor(83, 102, 129)
		if i == actualConsumptionCard {
			pdf.SetTextColor(220, 231, 247)
		}
		doc.text(x+12, y+11, 139, 8, item.label)
		pdf.SetTextColor(25, 51, 94)
		if i == actualConsumptionCard {
			pdf.SetTextColor(255, 255, 255)
		}
		doc.accounting(x+12, y+34, 139, 16, item.value)
		doc.right(x+12, y+60, 139, 8, snapshot.Currency.Code)
	}
	y += 96
	y -= 4
	pdf.SetTextColor(28, 43, 64)
	if snapshot.PDFTemplateVersion >= 18 {
		pdf.SetTextColor(89, 105, 127)
	}

	coverSummary := fmt.Sprintf("本期明细记录 %d 条  ·  金额统一保留两位小数。", snapshot.Total.Count)
	y = doc.text(40, y, 515, 9, coverSummary)
	y += 20
	y -= 6
	pdf.SetTextColor(89, 105, 127)
	y = doc.text(40, y, 515, 8.5, "对账说明 / RECONCILIATION NOTES")

	coverNote := "本文件用于核对本期服务消费，不是发票或支付凭证；退款金额已在实际消费金额中抵减。"
	y = doc.text(40, y+8, 515, 8.5, coverNote)

	discountNote := "本对账单中预扣金额、实际消费金额、退款金额已包含计费折扣，各项金额为折扣后的费用。金额按原始精度汇总后保留两位小数，展示数值相加可能存在尾差。"
	y = doc.text(40, y+6, 515, 8.5, discountNote)
	y = doc.text(40, y+6, 515, 8, "备注：异议期为账单生成后【5】个工作日内，逾期无书面异议即确认账单无误；扫描件有效。")

	if snapshot.HistoryImportID != "" {

		historyNote := "本期账单包含经核对的历史消费记录，已按消费日期归入本账期。"
		y = doc.text(40, y+6, 515, 8.5, historyNote)
	}

	if y > 767 {
		return nil, errors.New("billing cover identity exceeds available page space")
	}
	doc.headerV3(snapshot, 2, pages)
	doc.text(40, 119, 360, 18, "逐日消费明细")
	doc.right(380, 126, 175, 9, statement.Month+"  /  "+snapshot.Currency.Code)

	identityY := 154.0
	if identityHeight > 23 {
		identityY = 150
	}
	if snapshot.PDFTemplateVersion >= 18 {
		pdf.SetTextColor(89, 105, 127)
	}
	doc.text(40, identityY, 515, identitySize, dailyIdentity)
	pdf.SetFillColor(25, 51, 94)
	pdf.RectFromUpperLeftWithStyle(40, 179, 515, 26, "F")
	pdf.SetTextColor(255, 255, 255)

	amountFirstCol, amountLastCol := 2, 4
	if snapshot.PDFTemplateVersion >= 18 {
		amountLastCol = 5
	}
	columns := []float64{50, 125, 185, 274, 372, 460}
	widths := []float64{68, 53, 82, 91, 81, 85}
	titles := []string{"日期", "充值金额", "预扣金额", "实际消费金额", "退款金额", "账户余额"}

	for i, title := range titles {
		if i == 0 {
			doc.text(columns[i], 186, widths[i], 9, title)
		} else {
			doc.right(columns[i], 186, widths[i], 9, title)
		}
	}
	y = 205
	if len(snapshot.Days) > 31 {
		return nil, errors.New("billing month exceeds 31 rows")
	}
	zeroAmount, closingBalance := "0.00", "0.00"
	var dailyBalances []string
	if snapshot.PDFTemplateVersion >= 18 {
		dailyBalances, err = billingPostpaidDailyBalanceAmounts(snapshot.Days)
	} else {
		dailyBalances, err = billingPostpaidDailyBalances(snapshot.Days)
	}
	if err != nil {
		return nil, err
	}
	pdf.SetTextColor(35, 49, 69)
	doc.text(columns[0], y+3.5, widths[0], 8.2, "上期余额")
	doc.right(columns[1], y+3.5, widths[1], 8.2, zeroAmount)
	if snapshot.PDFTemplateVersion >= 18 {
		doc.accounting(columns[5], y+3.5, widths[5], 8.2, zeroAmount)
	} else {
		doc.right(columns[5], y+3.5, widths[5], 8.2, zeroAmount)
	}
	y += 15

	for i, row := range snapshot.Days {
		if i%2 == 0 {
			pdf.SetFillColor(245, 247, 251)
			pdf.RectFromUpperLeftWithStyle(40, y, 515, 15, "F")
		}
		pdf.SetTextColor(35, 49, 69)
		values := []string{row.Label, zeroAmount, "-", "未纳入", "-", "-"}
		if row.State != "outside_period" {
			closingBalance = dailyBalances[i]
			values = []string{row.Label, zeroAmount, row.Charge, row.Amount, row.Refund, dailyBalances[i]}
		}

		for col, value := range values {
			if col == 0 {
				doc.text(columns[col], y+3.5, widths[col], 8.2, value)
			} else {
				if col >= amountFirstCol && col <= amountLastCol && row.State != "outside_period" {
					doc.accounting(columns[col], y+3.5, widths[col], 8.2, value)
				} else {
					doc.right(columns[col], y+3.5, widths[col], 8.2, value)
				}
			}
		}
		y += 15
	}
	pdf.SetFillColor(227, 234, 245)
	pdf.RectFromUpperLeftWithStyle(40, y, 515, 25, "F")
	pdf.SetTextColor(25, 51, 94)
	totalValues := []string{"合计", zeroAmount, snapshot.Total.Charge, snapshot.Total.Amount, snapshot.Total.Refund, closingBalance}

	for col, value := range totalValues {
		if col == 0 {
			doc.text(columns[col], y+7, widths[col], 8.8, value)
		} else {
			if col >= amountFirstCol && col <= amountLastCol {
				doc.accounting(columns[col], y+7, widths[col], 8.8, value)
			} else {
				doc.right(columns[col], y+7, widths[col], 8.8, value)
			}
		}
	}
	pdf.SetTextColor(89, 105, 127)

	dailyNote := "实际消费金额 = 预扣金额 - 退款金额；账户余额 = 前日账户余额 - 当日实际消费金额。先用后结，上期余额和充值金额均为 0.00。余额按原始精度累计后保留两位小数。“未纳入”表示不在服务周期内。"
	y = doc.text(40, y+37, 515, 8, dailyNote)

	confirmationNote := "下发、异议及确认状态以平台记录为准。原始文件不随状态改变；确认信息见确认版首页。"
	y = doc.text(40, y+5, 515, 8, confirmationNote)
	if y > 767 {
		return nil, errors.New("billing daily notes exceed available page space")
	}
	var modelChargeRecords, modelRefundRecords, highestChargeQuota int64
	highestChargeShare := "0.00%"

	for _, row := range snapshot.Models {
		if row.ChargeCount < 0 || row.RefundCount < 0 ||
			row.ChargeCount > math.MaxInt64-modelChargeRecords ||
			row.RefundCount > math.MaxInt64-modelRefundRecords {
			return nil, errors.New("billing model record count exceeds exact limit")
		}
		modelChargeRecords += row.ChargeCount
		modelRefundRecords += row.RefundCount
		if row.ChargeQuota > highestChargeQuota {
			highestChargeQuota = row.ChargeQuota
			highestChargeShare = row.ChargeShare
		}
	}

	for pageIndex, modelRows := range modelPages {
		pageNumber := 3 + pageIndex
		doc.headerV3(snapshot, pageNumber, pages)

		title := "模型消费明细"
		if pageIndex > 0 {
			title += "（续）"
		}
		doc.text(40, 119, 360, 18, title)
		doc.right(380, 126, 175, 9, statement.Month+"  /  "+snapshot.Currency.Code)
		headerY, rowsY := 179.0, 205.0
		pdf.SetTextColor(89, 105, 127)

		modelIntro := "MODEL CONSUMPTION DETAILS  /  按模型核对预扣金额、实际消费金额及退款金额"
		doc.text(40, 151, 515, 7.5, modelIntro)
		doc.text(40, 165, 515, identitySize, modelIdentity)

		if pageIndex == 0 {
			summaryTop := 184.0 + modelHeadingOffset

			netLabel := "实际消费金额"

			for index, item := range []struct {
				label string
				value string
			}{
				{"模型数量", fmt.Sprintf("%d 个", billingModelCount(snapshot.Models))},
				{"消费笔数", fmt.Sprintf("%d 笔", modelChargeRecords)},
				{"退款笔数", fmt.Sprintf("%d 笔", modelRefundRecords)},
				{netLabel, snapshot.Currency.Symbol + " " + snapshot.Total.Amount},
			} {
				x := 40 + float64(index)*131
				pdf.SetFillColor(241, 245, 251)
				pdf.SetTextColor(83, 102, 129)
				if index == 3 {
					pdf.SetFillColor(25, 51, 94)
					pdf.SetTextColor(220, 231, 247)
				}
				pdf.RectFromUpperLeftWithStyle(x, summaryTop, 122, 46, "F")
				doc.text(x+10, summaryTop+8, 102, 7.2, item.label)
				pdf.SetTextColor(25, 51, 94)
				if index == 3 {
					pdf.SetTextColor(255, 255, 255)
				}
				if index == 3 {
					doc.accounting(x+10, summaryTop+24, 102, 11.5, snapshot.Total.Amount)
				} else {
					doc.right(x+10, summaryTop+24, 102, 11.5, item.value)
				}
			}
			headerY, rowsY = 245, 279
		} else {
			headerY, rowsY = 184, 218
		}
		headerY += modelHeadingOffset
		rowsY += modelHeadingOffset
		pdf.SetFillColor(25, 51, 94)

		headerHeight := 34.0
		pdf.RectFromUpperLeftWithStyle(40, headerY, 515, headerHeight, "F")
		pdf.SetTextColor(255, 255, 255)
		columns := []float64{46, 222, 309, 396, 482, 520}
		widths := []float64{168, 79, 79, 79, 31, 30}
		titles := []string{"模型名称", "预扣金额", "实际消费金额", "退款金额", "计费费率", "金额占比"}
		for i, title := range titles {
			if i == 0 {
				doc.text(columns[i], headerY+8, widths[i], 8.4, title)
			} else {
				doc.right(columns[i], headerY+8, widths[i], 8.4, title)
			}
		}
		y = rowsY
		if len(modelRows) == 0 {
			pdf.SetTextColor(89, 105, 127)
			y = doc.text(50, y+18, 495, 9, "本期没有模型消费记录。") + 16
		}
		for rowIndex, row := range modelRows {
			name := billingModelDisplayName(row)
			rowHeight, err := billingModelRowHeight(pdf, row, widths[0])
			if err != nil {
				return nil, err
			}
			if highestChargeQuota > 0 && row.ChargeQuota == highestChargeQuota {
				pdf.SetFillColor(234, 242, 252)
				pdf.RectFromUpperLeftWithStyle(40, y, 515, rowHeight, "F")
				pdf.SetFillColor(47, 103, 177)
				pdf.RectFromUpperLeftWithStyle(40, y, 3, rowHeight, "F")
			} else if rowIndex%2 == 0 {
				pdf.SetFillColor(245, 247, 251)
				pdf.RectFromUpperLeftWithStyle(40, y, 515, rowHeight, "F")
			}
			pdf.SetTextColor(35, 49, 69)
			nameEnd := doc.text(columns[0], y+4, widths[0], 8.2, name)
			if row.ActiveDays > 0 {
				first := time.Unix(row.FirstPosted, 0).In(billingLocation).Format("01-02")
				last := time.Unix(row.LastPosted, 0).In(billingLocation).Format("01-02")
				usage := fmt.Sprintf("%d 天 · %s–%s", row.ActiveDays, first, last)
				pdf.SetTextColor(89, 105, 127)
				doc.text(columns[0], nameEnd+1, widths[0], 6.8, usage)
				pdf.SetTextColor(35, 49, 69)
			}
			values := []string{row.Charge, row.Amount, row.Refund, billingHistoricalRateLabel(row), row.ChargeShare}
			columnOffset := 1

			for col, value := range values {
				size := 7.0

				if col < 3 {
					doc.accounting(columns[col+columnOffset], y+5, widths[col+columnOffset], size, value)
				} else {
					doc.right(columns[col+columnOffset], y+5, widths[col+columnOffset], size, value)
				}
			}

			shareColumn := len(columns) - 1
			barX, barY, barWidth := columns[shareColumn]+18, y+21, widths[shareColumn]-18
			pdf.SetFillColor(221, 228, 237)
			pdf.RectFromUpperLeftWithStyle(barX, barY, barWidth, 2.5, "F")
			share := 0.0
			if snapshot.ChargeQuota > 0 && row.ChargeQuota > 0 {
				share = math.Min(1, float64(row.ChargeQuota)/float64(snapshot.ChargeQuota))
			}
			pdf.SetFillColor(47, 103, 177)
			pdf.RectFromUpperLeftWithStyle(barX, barY, barWidth*share, 2.5, "F")
			y += rowHeight
		}
		pdf.SetTextColor(89, 105, 127)
		if pageIndex == len(modelPages)-1 {
			pdf.SetFillColor(227, 234, 245)
			pdf.RectFromUpperLeftWithStyle(40, y, 515, 25, "F")
			pdf.SetTextColor(25, 51, 94)

			totalShare := "0.00%"
			if snapshot.ChargeQuota > 0 {
				totalShare = "100.00%"
			}
			totalValues := []string{"合计", snapshot.Total.Charge, snapshot.Total.Amount, snapshot.Total.Refund, "—", totalShare}

			amountStart, amountEnd := 1, 3

			for col, value := range totalValues {
				if col == 0 {
					doc.text(columns[col], y+7, widths[col], 8.8, value)
				} else {
					size := 7.0

					if col >= amountStart && col <= amountEnd {
						doc.accounting(columns[col], y+7, widths[col], size, value)
					} else {
						doc.right(columns[col], y+7, widths[col], size, value)
					}
				}
			}
			pdf.SetTextColor(89, 105, 127)
			y = doc.text(40, y+37, 515, 8, modelNote)

			if highestChargeQuota > 0 {
				pdf.SetTextColor(47, 103, 177)

				highlightNote := "重点核对：浅蓝标记项为本期预扣金额最高的模型，占本期预扣金额 " + highestChargeShare + "。"
				y = doc.text(40, y+4, 515, 8, highlightNote)
			}
			pdf.SetTextColor(89, 105, 127)

			reconcileNote := "各模型预扣金额、实际消费金额及退款金额按原始精度汇总，与本期对账单一致。"
			y = doc.text(40, y+4, 515, 8, reconcileNote)

		} else {

			continuation := "模型消费明细续下页，所有分页共同组成完整明细。"
			y = doc.text(40, y+12, 515, 8, continuation)
		}
		if y > 767 {
			return nil, errors.New("billing model summary exceeds available page space")
		}
	}

	if doc.err != nil {
		return nil, doc.err
	}
	if missingGlyph {
		return nil, errors.New("company information contains characters unsupported by the PDF font")
	}
	return pdf.GetBytesPdfReturnErr()
}
