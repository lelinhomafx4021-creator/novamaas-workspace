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

// The frozen creation time and document number help readers locate a statement.
// Verification still uses the complete, unmodified archive fingerprints.
func (doc *billingPDFDocument) verificationInfo(y float64, statement *model.BillingStatement, fingerprints []struct{ label, digest string }) float64 {
	if doc.err != nil {
		return y
	}
	height := 76 + float64(len(fingerprints))*30
	fingerprintTop, fingerprintGap, labelWidth := 58.0, 30.0, 491.0
	compact := y+height > 767
	if compact {
		// The document number is already shown above. Omit its duplicate when
		// long corporate identities need the original page space.
		height = 56 + float64(len(fingerprints))*14
		fingerprintTop, fingerprintGap, labelWidth = 38, 14, 123
	}
	doc.pdf.SetFillColor(245, 247, 251)
	doc.pdf.RectFromUpperLeftWithStyle(40, y, 515, height, "F")
	doc.pdf.SetTextColor(25, 51, 94)
	doc.text(52, y+7, 491, 9, "文件核验信息")
	doc.pdf.SetTextColor(89, 105, 127)
	created := time.Unix(statement.CreatedAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")
	doc.text(52, y+22, 491, 8, fmt.Sprintf("生成时间  %s（北京时间）  /  账期 %s  /  版本 %02d", created, statement.Month, statement.Revision))
	if !compact {
		doc.text(52, y+40, 491, 8, "文件编号  "+statement.ID)
	}
	for i, item := range fingerprints {
		top := y + fingerprintTop + float64(i)*fingerprintGap
		doc.pdf.SetTextColor(89, 105, 127)
		doc.text(52, top, labelWidth, 8, item.label)
		doc.pdf.SetTextColor(35, 49, 69)
		if compact {
			doc.text(181, top, 362, 8, item.digest)
		} else {
			doc.text(52, top+13, 491, 8, item.digest)
		}
	}
	doc.pdf.SetTextColor(89, 105, 127)
	doc.text(52, y+height-16, 491, 7.2, "核验时请与平台归档的完整指纹比对；时间和编号用于定位账单。")
	return y + height
}

// Keep v1/v2 renderers intact: a retry or receipt for an existing document
// must reproduce its frozen template, even after a branding setting changes.
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
	platformTop, titleTop := 33.0, 34.0
	if snapshot.PDFTemplateVersion == 6 || snapshot.PDFTemplateVersion >= 9 {
		platformTop, titleTop = 42, 43
	}
	if doc.err = doc.pdf.ImageByHolder(mark, 40+(38-width)/2, platformTop+(38-height)/2, &gopdf.Rect{W: width, H: height}); doc.err != nil {
		return
	}
	doc.pdf.SetTextColor(25, 51, 94)
	platformWidth := 465.0
	if snapshot.PDFTemplateVersion >= 4 && (len(snapshot.PDFOperatingLogoPNG) > 0 || snapshot.OperatingName != "") {
		platformWidth = 335
	}
	end := doc.text(90, titleTop, platformWidth, 15, snapshot.Issuer)
	if end > 80 {
		doc.err = errors.New("platform name exceeds PDF letterhead")
		return
	}
	doc.pdf.SetTextColor(90, 106, 129)
	if (snapshot.PDFTemplateVersion == 6 || snapshot.PDFTemplateVersion >= 9) && platformWidth < 465 {
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
	} else if snapshot.PDFTemplateVersion == 5 && platformWidth < 465 {
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
			if doc.err = doc.pdf.ImageByHolder(operatingMark, 555-width, 34+(40-height)/2, &gopdf.Rect{W: width, H: height}); doc.err != nil {
				return
			}
		}
		label := snapshot.OperatingName
		if label == "" {
			label = "运营主体 / OPERATOR"
		}
		labelY := 79.0
		if len(snapshot.PDFOperatingLogoPNG) == 0 {
			labelY = 47
		}
		doc.right(430, labelY, 125, 8, label)
	} else if snapshot.PDFTemplateVersion == 4 && platformWidth < 465 {
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
			scale := math.Min(34/float64(config.Width), 34/float64(config.Height))
			width, height := float64(config.Width)*scale, float64(config.Height)*scale
			if doc.err = doc.pdf.ImageByHolder(operatingMark, 518+(34-width)/2, 33+(34-height)/2, &gopdf.Rect{W: width, H: height}); doc.err != nil {
				return
			}
		}
		label := snapshot.OperatingName
		if label == "" {
			label = "运营主体 / OPERATOR"
		}
		if doc.text(430, 76, 125, 7, label) > 98 {
			doc.err = errors.New("operating entity name exceeds PDF letterhead")
			return
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

func renderBillingStatementPDFV4(statement *model.BillingStatement, snapshot *BillingSnapshot, receipt bool) ([]byte, error) {
	return renderBillingStatementPDFV3(statement, snapshot, receipt)
}

func renderBillingStatementPDFV5(statement *model.BillingStatement, snapshot *BillingSnapshot, receipt bool) ([]byte, error) {
	return renderBillingStatementPDFV3(statement, snapshot, receipt)
}

func renderBillingStatementPDFV6(statement *model.BillingStatement, snapshot *BillingSnapshot, receipt bool) ([]byte, error) {
	return renderBillingStatementPDFV3(statement, snapshot, receipt)
}

func renderBillingStatementPDFV7(statement *model.BillingStatement, snapshot *BillingSnapshot, receipt bool) ([]byte, error) {
	return renderBillingStatementPDFV3(statement, snapshot, receipt)
}

func renderBillingStatementPDFV8(statement *model.BillingStatement, snapshot *BillingSnapshot, receipt bool) ([]byte, error) {
	return renderBillingStatementPDFV3(statement, snapshot, receipt)
}

func billingModelSummaryPages(pdf *gopdf.GoPdf, rows []BillingModelRow, nameWidth float64, templateVersion int) ([][]BillingModelRow, error) {
	if len(rows) == 0 {
		return [][]BillingModelRow{{}}, nil
	}
	if err := pdf.SetFont("billing", "", 8.2); err != nil {
		return nil, err
	}
	pages := make([][]BillingModelRow, 0, 1)
	page := make([]BillingModelRow, 0, 24)
	height := 0.0
	pageHeight := 475.0
	if templateVersion >= 8 {
		// V8 reserves a compact executive summary above the table and a
		// reconciliation note below it. Keep pagination deterministic so the
		// final row, totals and notes never compete for the footer area.
		pageHeight = 365
	}
	if templateVersion >= 9 {
		pageHeight = 315
	}
	for _, row := range rows {
		name := billingModelDisplayName(row, templateVersion)
		lines, err := pdf.SplitText(name, nameWidth)
		if err != nil {
			return nil, err
		}
		rowHeight := math.Max(20, float64(len(lines))*8.2*1.45+8)
		if templateVersion >= 8 && row.ActiveDays > 0 {
			rowHeight += 13
		}
		if templateVersion >= 8 {
			rowHeight = math.Max(32, rowHeight)
		}
		if rowHeight > pageHeight {
			return nil, errors.New("billing model name exceeds available page space")
		}
		if height+rowHeight > pageHeight && len(page) > 0 {
			pages = append(pages, page)
			page = make([]BillingModelRow, 0, 24)
			height = 0
		}
		page = append(page, row)
		height += rowHeight
	}
	return append(pages, page), nil
}

func renderBillingStatementPDFV3(statement *model.BillingStatement, snapshot *BillingSnapshot, receipt bool) ([]byte, error) {
	accountingSymbol := snapshot.Currency.Symbol
	if snapshot.PDFTemplateVersion >= 14 {
		accountingSymbol = ""
	}
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
	modelPages := make([][]BillingModelRow, 0)
	if snapshot.PDFTemplateVersion >= 7 {
		nameWidth := 195.0
		if snapshot.PDFTemplateVersion >= 8 {
			nameWidth = 147
		}
		if snapshot.PDFTemplateVersion >= 9 {
			nameWidth = 113
			if snapshot.PDFTemplateVersion >= 10 {
				nameWidth = 144
			}
			if snapshot.PDFTemplateVersion >= 13 {
				nameWidth = 108
			}
		}
		var pageErr error
		modelPages, pageErr = billingModelSummaryPages(pdf, snapshot.Models, nameWidth, snapshot.PDFTemplateVersion)
		if pageErr != nil {
			return nil, pageErr
		}
	}
	pages := 2 + len(modelPages)
	if receipt {
		pages++
	}
	doc := billingPDFDocument{pdf: pdf}
	doc.headerV3(snapshot, 1, pages)
	pdf.SetTextColor(89, 105, 127)
	doc.text(40, 121, 360, 8.5, "STATEMENT OF ACCOUNT")
	pdf.SetTextColor(25, 51, 94)
	doc.text(40, 141, 350, 26, "月度对账单")
	if snapshot.PDFTemplateVersion >= 17 {
		month, err := time.Parse("2006-01", statement.Month)
		if err != nil {
			return nil, err
		}
		pdf.SetFillColor(241, 245, 251)
		pdf.RectFromUpperLeftWithStyle(370, 133, 185, 45, "F")
		doc.right(382, 143, 161, 24, month.Format("2006年01月"))
	} else {
		doc.right(410, 144, 145, 22, statement.Month)
	}
	pdf.SetTextColor(88, 104, 126)
	doc.text(40, 185, 515, 8.5, fmt.Sprintf("文件编号  %s   /   版本 %02d", statement.ID, statement.Revision))
	doc.text(40, 202, 515, 8.5, "生成时间  "+time.Unix(statement.CreatedAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+"   /   Asia/Shanghai (UTC+08:00)")

	pdf.SetTextColor(89, 105, 127)
	doc.text(40, 242, 232, 8.5, "服务平台 / SERVICE PROVIDER")
	doc.text(308, 242, 247, 8.5, "对账客户 / BILL TO")
	pdf.SetTextColor(28, 43, 64)
	left := doc.text(40, 264, 232, 12, snapshot.Issuer)
	left = doc.text(40, left+14, 232, 9, "账期起点  "+time.Unix(statement.StartAt, 0).In(billingLocation).Format("2006-01-02 15:04:05"))
	left = doc.text(40, left+5, 232, 9, "账期终点  "+time.Unix(statement.EndAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+" (不含)")
	currencyLabel := "计价币种  " + snapshot.Currency.Code + "  /  " + snapshot.Currency.Symbol
	if snapshot.PDFTemplateVersion >= 14 {
		currencyLabel = "计价币种  " + snapshot.Currency.Code
	}
	left = doc.text(40, left+5, 232, 9, currencyLabel)
	right := doc.text(308, 264, 247, 10, snapshot.CompanyTitle)
	right = doc.text(308, right+12, 247, 9, fmt.Sprintf("客户账户  %s (#%d)", snapshot.Username, statement.UserID))
	if snapshot.DisplayName != "" && snapshot.DisplayName != snapshot.CompanyTitle {
		right = doc.text(308, right+4, 247, 9, "账户名称  "+snapshot.DisplayName)
	}
	right = doc.text(308, right+4, 247, 9, "纳税人识别号  "+snapshot.TaxID)
	y := max(left, right) + 27
	for i, item := range []struct{ label, value string }{
		{"消费总额 / CHARGES", snapshot.Total.Charge},
		{"退款总额 / REFUNDS", snapshot.Total.Refund},
		{"本期净额 / NET TOTAL", snapshot.Total.Amount},
	} {
		x := 40 + float64(i)*176
		pdf.SetFillColor(241, 245, 251)
		if i == 2 {
			pdf.SetFillColor(25, 51, 94)
		}
		pdf.RectFromUpperLeftWithStyle(x, y, 163, 79, "F")
		pdf.SetTextColor(83, 102, 129)
		if i == 2 {
			pdf.SetTextColor(220, 231, 247)
		}
		doc.text(x+12, y+11, 139, 8, item.label)
		pdf.SetTextColor(25, 51, 94)
		if i == 2 {
			pdf.SetTextColor(255, 255, 255)
		}
		if snapshot.PDFTemplateVersion >= 13 {
			doc.accounting(x+12, y+34, 139, 16, item.value, accountingSymbol)
		} else {
			doc.right(x+12, y+34, 139, 16, item.value)
		}
		doc.right(x+12, y+60, 139, 8, snapshot.Currency.Code)
	}
	y += 96
	pdf.SetTextColor(28, 43, 64)
	coverSummary := fmt.Sprintf("本期明细记录 %d 条  ·  按原始整数额度汇总，金额保留六位小数。", snapshot.Total.Count)
	if snapshot.PDFTemplateVersion >= 8 {
		coverSummary = fmt.Sprintf("本期明细记录 %d 条  ·  金额统一保留六位小数。", snapshot.Total.Count)
	}
	y = doc.text(40, y, 515, 9, coverSummary)
	y += 20
	pdf.SetTextColor(89, 105, 127)
	y = doc.text(40, y, 515, 8.5, "对账说明 / RECONCILIATION NOTES")
	coverNote := "本文件用于平台与客户核对服务消费，不是银行凭证、发票或支付收据。管理员充值及临时预扣不计入消费；退款抵减净额，不代表再次支付。"
	if snapshot.PDFTemplateVersion >= 8 {
		coverNote = "本文件用于核对本期服务消费，不是发票或支付凭证；退款已在本期净额中抵减。"
	}
	y = doc.text(40, y+8, 515, 8.5, coverNote)
	if snapshot.PDFTemplateVersion >= 12 {
		y = doc.text(40, y+6, 515, 8.5, billingDiscountAmountNote)
	}
	if snapshot.HistoryImportID != "" {
		historyNote := "本期数据为经管理员核验的历史日志补录，按原日志时间归属账期，未改变钱包余额。导入批次：" + snapshot.HistoryImportID + "。"
		if snapshot.PDFTemplateVersion >= 8 {
			historyNote = "本期账单包含经核对的历史消费记录，已按消费日期归入本账期。"
		}
		y = doc.text(40, y+6, 515, 8.5, historyNote)
	}
	y += 18
	if snapshot.PDFTemplateVersion >= 16 {
		if statement.ExcelSHA256 == "" {
			return nil, errors.New("billing Excel fingerprint is required")
		}
		y = doc.verificationInfo(y, statement, []struct{ label, digest string }{
			{"账单数据指纹 / SHA-256", statement.SnapshotSHA256},
			{"配套 Excel 指纹 / SHA-256", statement.ExcelSHA256},
		})
	} else {
		digestLabel := "冻结数据快照 / SHA-256"
		if snapshot.PDFTemplateVersion >= 8 {
			digestLabel = "账单数据校验码 / SHA-256"
		}
		doc.text(40, y, 515, 8, digestLabel)
		y = doc.text(40, y+17, 515, 8, statement.SnapshotSHA256)
		if snapshot.PDFTemplateVersion >= 9 {
			if statement.ExcelSHA256 == "" {
				return nil, errors.New("billing Excel fingerprint is required")
			}
			y = doc.text(40, y+9, 515, 8, "配套 Excel / SHA-256")
			y = doc.text(40, y+6, 515, 8, statement.ExcelSHA256)
			y = doc.text(40, y+6, 515, 7.5, "请核对配套 Excel 指纹；客户确认的校验清单同时绑定 PDF、Excel 和账单数据。")
		}
	}
	if y > 767 {
		return nil, errors.New("billing cover identity exceeds available page space")
	}

	doc.headerV3(snapshot, 2, pages)
	doc.text(40, 119, 360, 18, "逐日消费明细")
	doc.right(380, 126, 175, 9, statement.Month+"  /  "+snapshot.Currency.Code)
	doc.text(40, 154, 515, 8.5, fmt.Sprintf("客户 %s (#%d)  ·  版本 %02d  ·  Asia/Shanghai", snapshot.Username, statement.UserID, statement.Revision))
	pdf.SetFillColor(25, 51, 94)
	pdf.RectFromUpperLeftWithStyle(40, 179, 515, 26, "F")
	pdf.SetTextColor(255, 255, 255)
	isPostpaid := snapshot.PDFTemplateVersion >= 15
	amountFirstCol, amountLastCol := 1, 3
	columns := []float64{50, 147, 257, 367, 477}
	widths := []float64{85, 98, 98, 98, 68}
	titles := []string{"日期", "消费金额", "退款金额", "净消费金额", "记录数"}
	if isPostpaid {
		amountFirstCol, amountLastCol = 2, 4
		columns = []float64{50, 125, 185, 274, 372, 460}
		widths = []float64{68, 53, 82, 91, 81, 85}
		titles = []string{"日期", "充值金额", "预扣金额", "实际消费金额", "退款金额", "账户余额"}
	}
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
	var dailyBalances []string
	closingBalance := "0.000000"
	if isPostpaid {
		var err error
		dailyBalances, err = billingPostpaidDailyBalances(snapshot.Days)
		if err != nil {
			return nil, err
		}
		pdf.SetTextColor(35, 49, 69)
		doc.text(columns[0], y+3.5, widths[0], 8.2, "上期余额")
		doc.right(columns[1], y+3.5, widths[1], 8.2, "0")
		doc.right(columns[5], y+3.5, widths[5], 8.2, "0")
		y += 15
	}
	for i, row := range snapshot.Days {
		if i%2 == 0 {
			pdf.SetFillColor(245, 247, 251)
			pdf.RectFromUpperLeftWithStyle(40, y, 515, 15, "F")
		}
		pdf.SetTextColor(35, 49, 69)
		values := []string{row.Label, row.Charge, row.Refund, row.Amount, fmt.Sprint(row.Count)}
		if isPostpaid {
			values = []string{row.Label, "0", "-", "未纳入", "-", "-"}
			if row.State != "outside_period" {
				closingBalance = dailyBalances[i]
				values = []string{row.Label, "0", row.Charge, row.Amount, row.Refund, dailyBalances[i]}
			}
		} else if row.State == "outside_period" {
			values = []string{row.Label, "-", "-", "未纳入", "-"}
		}
		for col, value := range values {
			if col == 0 {
				doc.text(columns[col], y+3.5, widths[col], 8.2, value)
			} else {
				if snapshot.PDFTemplateVersion >= 13 && col >= amountFirstCol && col <= amountLastCol && row.State != "outside_period" {
					doc.accounting(columns[col], y+3.5, widths[col], 8.2, value, accountingSymbol)
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
	totalValues := []string{"合计", snapshot.Total.Charge, snapshot.Total.Refund, snapshot.Total.Amount, fmt.Sprint(snapshot.Total.Count)}
	if isPostpaid {
		totalValues = []string{"合计", "0", snapshot.Total.Charge, snapshot.Total.Amount, snapshot.Total.Refund, closingBalance}
	}
	for col, value := range totalValues {
		if col == 0 {
			doc.text(columns[col], y+7, widths[col], 8.8, value)
		} else {
			if snapshot.PDFTemplateVersion >= 13 && col >= amountFirstCol && col <= amountLastCol {
				doc.accounting(columns[col], y+7, widths[col], 8.8, value, accountingSymbol)
			} else {
				doc.right(columns[col], y+7, widths[col], 8.8, value)
			}
		}
	}
	pdf.SetTextColor(89, 105, 127)
	dailyNote := "未纳入：正式记账起点之前。明细记录数包含消费及退款；完整逐笔明细可从平台下载经校验的归档文件。"
	if snapshot.PDFTemplateVersion >= 8 {
		dailyNote = "“未纳入”表示该日期不在本账单服务周期内；记录数包含消费和退款。详细记录可在平台下载。"
	}
	if isPostpaid {
		dailyNote = "实际消费金额 = 预扣金额 - 退款金额；当日账户余额 = 前日账户余额 - 当日实际消费金额。先用后结，上期余额和充值金额均为 0。“未纳入”表示不在本账单服务周期内。"
	}
	y = doc.text(40, y+37, 515, 8, dailyNote)
	y = doc.text(40, y+5, 515, 8, "下发、异议及确认状态以平台记录为准。原始文件不随状态改变；确认后另附回执。")
	if y > 767 {
		return nil, errors.New("billing daily notes exceed available page space")
	}
	isV8 := snapshot.PDFTemplateVersion >= 8
	var modelChargeRecords, modelRefundRecords, highestChargeQuota int64
	highestChargeShare := "0.00%"
	if isV8 {
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
	}
	for pageIndex, modelRows := range modelPages {
		pageNumber := 3 + pageIndex
		doc.headerV3(snapshot, pageNumber, pages)
		title := "按模型消费汇总"
		if isV8 {
			title = "模型消费明细"
			if pageIndex > 0 {
				title += "（续）"
			}
		}
		doc.text(40, 119, 360, 18, title)
		doc.right(380, 126, 175, 9, statement.Month+"  /  "+snapshot.Currency.Code)
		headerY, rowsY := 179.0, 205.0
		if isV8 {
			pdf.SetTextColor(89, 105, 127)
			doc.text(40, 151, 515, 7.5, "MODEL CONSUMPTION DETAILS  /  按模型核对本期消费、退款及净额")
			doc.text(40, 165, 515, 8.2, fmt.Sprintf("客户 %s (#%d)  ·  逐项列示模型金额、涉及账单日和消费金额占比", snapshot.Username, statement.UserID))
			if pageIndex == 0 {
				summaryTop := 184.0
				for index, item := range []struct {
					label string
					value string
				}{
					{"模型数量", fmt.Sprintf("%d 个", billingModelCount(snapshot.Models))},
					{"消费笔数", fmt.Sprintf("%d 笔", modelChargeRecords)},
					{"退款笔数", fmt.Sprintf("%d 笔", modelRefundRecords)},
					{"本期净额", snapshot.Currency.Symbol + " " + snapshot.Total.Amount},
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
					if snapshot.PDFTemplateVersion >= 13 && index == 3 {
						doc.accounting(x+10, summaryTop+24, 102, 11.5, snapshot.Total.Amount, accountingSymbol)
					} else {
						doc.right(x+10, summaryTop+24, 102, 11.5, item.value)
					}
				}
				headerY, rowsY = 245, 279
			} else {
				headerY, rowsY = 184, 218
			}
		} else {
			doc.text(40, 154, 515, 8.5, fmt.Sprintf("客户 %s (#%d)  ·  仅含客户消费金额，不含渠道、上游成本或利润", snapshot.Username, statement.UserID))
		}
		pdf.SetFillColor(25, 51, 94)
		headerHeight := 28.0
		if isV8 {
			headerHeight = 34
		}
		pdf.RectFromUpperLeftWithStyle(40, headerY, 515, headerHeight, "F")
		pdf.SetTextColor(255, 255, 255)
		columns := []float64{50, 257, 345, 423, 507}
		widths := []float64{195, 76, 66, 72, 38}
		titles := []string{"模型名称", "消费金额", "退款金额", "净消费", "记录数"}
		if isV8 {
			columns = []float64{50, 202, 261, 335, 402, 478}
			widths = []float64{147, 54, 69, 62, 71, 67}
			titles = []string{"模型 / 涉及账单日", "", "消费金额", "退款金额", "本期净额", "消费金额占比"}
		}
		if snapshot.PDFTemplateVersion >= 9 {
			columns = []float64{46, 164, 210, 274, 338, 402, 457, 502}
			widths = []float64{113, 40, 59, 59, 59, 50, 40, 47}
			titles = []string{"模型 / 账单日", "", "消费金额", "退款金额", "本期净额", "计费费率", "原价", "金额占比"}
		}
		if snapshot.PDFTemplateVersion >= 10 {
			columns = []float64{46, 195, 243, 311, 379, 449, 508}
			widths = []float64{144, 42, 63, 63, 65, 54, 41}
			titles = []string{"模型 / 计费组", "", "消费金额", "退款金额", "本期净额", "计费费率", "金额占比"}
		}
		if snapshot.PDFTemplateVersion >= 11 {
			titles[0] = "模型名称"
		}
		if snapshot.PDFTemplateVersion >= 13 {
			columns = []float64{46, 160, 199, 290, 381, 472, 514}
			widths = []float64{108, 33, 86, 86, 86, 36, 36}
		}
		for i, title := range titles {
			if i == 0 {
				doc.text(columns[i], headerY+8, widths[i], 8.4, title)
			} else {
				doc.right(columns[i], headerY+8, widths[i], 8.4, title)
			}
		}
		if isV8 {
			doc.right(columns[1], headerY+5, widths[1], 7.6, "笔数")
			doc.right(columns[1], headerY+17, widths[1], 6.8, "消费 / 退款")
		}
		y = rowsY
		if len(modelRows) == 0 {
			pdf.SetTextColor(89, 105, 127)
			y = doc.text(50, y+18, 495, 9, "本期没有模型消费记录。") + 16
		}
		for rowIndex, row := range modelRows {
			name := billingModelDisplayName(row, snapshot.PDFTemplateVersion)
			if err := pdf.SetFont("billing", "", 8.2); err != nil {
				return nil, err
			}
			lines, err := pdf.SplitText(name, widths[0])
			if err != nil {
				return nil, err
			}
			rowHeight := math.Max(20, float64(len(lines))*8.2*1.45+8)
			if isV8 && row.ActiveDays > 0 {
				rowHeight += 13
			}
			if isV8 {
				rowHeight = math.Max(32, rowHeight)
			}
			if isV8 && highestChargeQuota > 0 && row.ChargeQuota == highestChargeQuota {
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
			if isV8 && row.ActiveDays > 0 {
				first := time.Unix(row.FirstPosted, 0).In(billingLocation).Format("01-02")
				last := time.Unix(row.LastPosted, 0).In(billingLocation).Format("01-02")
				usage := fmt.Sprintf("涉及 %d 个账单日 · %s", row.ActiveDays, first)
				if first != last {
					usage += " 至 " + last
				}
				if snapshot.PDFTemplateVersion >= 9 {
					usage = fmt.Sprintf("%d 天 · %s–%s", row.ActiveDays, first, last)
				}
				pdf.SetTextColor(89, 105, 127)
				doc.text(columns[0], nameEnd+1, widths[0], 6.8, usage)
				pdf.SetTextColor(35, 49, 69)
			}
			values := []string{row.Charge, row.Refund, row.Amount, fmt.Sprint(row.Count)}
			columnOffset := 1
			if isV8 {
				doc.right(columns[1], y+4, widths[1], 7.3, fmt.Sprintf("消费 %d", row.ChargeCount))
				doc.right(columns[1], y+16, widths[1], 7.3, fmt.Sprintf("退款 %d", row.RefundCount))
				values = []string{row.Charge, row.Refund, row.Amount, row.ChargeShare}
				columnOffset = 2
			}
			if snapshot.PDFTemplateVersion >= 9 {
				values = []string{row.Charge, row.Refund, row.Amount, billingRateLabel(row), "未记录", row.ChargeShare}
				if snapshot.PDFTemplateVersion >= 10 {
					values = []string{row.Charge, row.Refund, row.Amount, billingHistoricalRateLabel(row), row.ChargeShare}
				}
			}
			for col, value := range values {
				size := 8.2
				if snapshot.PDFTemplateVersion >= 9 {
					size = 7
				}
				if snapshot.PDFTemplateVersion >= 13 && col < 3 {
					doc.accounting(columns[col+columnOffset], y+5, widths[col+columnOffset], size, value, accountingSymbol)
				} else {
					doc.right(columns[col+columnOffset], y+5, widths[col+columnOffset], size, value)
				}
			}
			if isV8 {
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
			}
			y += rowHeight
		}
		pdf.SetTextColor(89, 105, 127)
		if pageIndex == len(modelPages)-1 {
			pdf.SetFillColor(227, 234, 245)
			pdf.RectFromUpperLeftWithStyle(40, y, 515, 25, "F")
			pdf.SetTextColor(25, 51, 94)
			totalValues := []string{"合计", snapshot.Total.Charge, snapshot.Total.Refund, snapshot.Total.Amount, fmt.Sprint(snapshot.Total.Count)}
			if isV8 {
				totalShare := "0.00%"
				if snapshot.ChargeQuota > 0 {
					totalShare = "100.00%"
				}
				totalValues = []string{"合计", "", snapshot.Total.Charge, snapshot.Total.Refund, snapshot.Total.Amount, totalShare}
				doc.right(columns[1], y+3, widths[1], 7.3, fmt.Sprintf("消费 %d", modelChargeRecords))
				doc.right(columns[1], y+14, widths[1], 7.3, fmt.Sprintf("退款 %d", modelRefundRecords))
			}
			if snapshot.PDFTemplateVersion >= 9 {
				if snapshot.PDFTemplateVersion >= 10 {
					totalValues = append(totalValues[:5], "—", totalValues[5])
				} else {
					totalValues = append(totalValues[:5], "—", "未记录", totalValues[5])
				}
			}
			for col, value := range totalValues {
				if col == 0 {
					doc.text(columns[col], y+7, widths[col], 8.8, value)
				} else {
					size := 8.8
					if snapshot.PDFTemplateVersion >= 9 {
						size = 7
					}
					if snapshot.PDFTemplateVersion >= 13 && col >= 2 && col <= 4 {
						doc.accounting(columns[col], y+7, widths[col], size, value, accountingSymbol)
					} else {
						doc.right(columns[col], y+7, widths[col], size, value)
					}
				}
			}
			pdf.SetTextColor(89, 105, 127)
			note := "记录数为正式账本明细数量，不等同于成功请求数；退款按实际入账模型抵减。"
			if isV8 {
				note = "本期净额 = 消费金额 - 退款金额；消费金额占比 = 单模型消费金额 ÷ 本期消费总额。"
			}
			if snapshot.PDFTemplateVersion >= 9 {
				if snapshot.PDFTemplateVersion >= 10 {
					note += " 同一模型按消费时实际计费组及费率分行列示；未记录表示历史证据缺失。"
					if snapshot.PDFTemplateVersion >= 11 {
						note += " 实际计费组详见随附 Excel。"
					}
				} else {
					note += " 计费费率为生成时当前可用分组参考，范围表示多组费率，详见 Excel。"
				}
			}
			y = doc.text(40, y+37, 515, 8, note)
			if snapshot.PDFTemplateVersion == 9 {
				y = doc.text(40, y+4, 515, 8, "费率参考时间："+time.Unix(snapshot.RateReferenceAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+"；正式账本未记录历史原价，不按当前费率倒推。")
			}
			if isV8 {
				if highestChargeQuota > 0 {
					pdf.SetTextColor(47, 103, 177)
					y = doc.text(40, y+4, 515, 8, "重点核对：浅蓝标记项为本期消费金额最高的模型，占本期消费金额 "+highestChargeShare+"。")
				}
				pdf.SetTextColor(89, 105, 127)
				y = doc.text(40, y+4, 515, 8, "各模型消费、退款及本期净额合计与本期对账单一致。")
			}
		} else {
			continuation := "模型消费汇总续下页。"
			if isV8 {
				continuation = "模型消费明细续下页，所有分页共同组成完整明细。"
			}
			y = doc.text(40, y+12, 515, 8, continuation)
		}
		if y > 767 {
			return nil, errors.New("billing model summary exceeds available page space")
		}
	}
	if receipt {
		doc.headerV3(snapshot, 3+len(modelPages), pages)
		doc.text(40, 122, 515, 23, "对账确认回执")
		pdf.SetTextColor(89, 105, 127)
		doc.text(40, 163, 515, 9, "CONFIRMATION RECEIPT  /  原始对账单附页")
		pdf.SetFillColor(237, 246, 241)
		pdf.RectFromUpperLeftWithStyle(40, 204, 515, 63, "F")
		pdf.SetTextColor(30, 102, 74)
		doc.text(56, 218, 483, 16, "客户账户已执行确认")
		doc.text(56, 244, 483, 9, "确认时间  "+time.Unix(statement.ConfirmedAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")+"  Asia/Shanghai")
		pdf.SetTextColor(28, 43, 64)
		y = doc.text(40, 297, 515, 10, fmt.Sprintf("确认账户  %s (#%d)", snapshot.Username, statement.UserID))
		y = doc.text(40, y+12, 515, 10, "对账主体  "+snapshot.CompanyTitle)
		y = doc.text(40, y+12, 515, 10, "纳税人识别号  "+snapshot.TaxID)
		y = doc.text(40, y+12, 515, 10, fmt.Sprintf("关联文件  %s  /  %s  /  版本 %02d", statement.ID, statement.Month, statement.Revision))
		if snapshot.PDFTemplateVersion >= 13 {
			doc.accounting(170, y+12, 385, 13, snapshot.Total.Amount, accountingSymbol)
			y = doc.text(40, y+12, 120, 13, "确认净额")
		} else {
			y = doc.text(40, y+12, 515, 13, "确认净额  "+snapshot.Currency.Code+" "+snapshot.Total.Amount)
		}
		y += 28
		pdf.SetTextColor(89, 105, 127)
		digests := []struct{ label, digest string }{{"原始 PDF / SHA-256", statement.PDFSHA256}, {"明细归档清单 / SHA-256", statement.ManifestSHA256}, {"冻结数据快照 / SHA-256", statement.SnapshotSHA256}}
		if snapshot.PDFTemplateVersion >= 16 {
			digests = []struct{ label, digest string }{
				{"原始对账单指纹 / SHA-256", statement.PDFSHA256},
				{"关联文件清单指纹 / SHA-256", statement.ManifestSHA256},
				{"账单数据指纹 / SHA-256", statement.SnapshotSHA256},
			}
		} else if snapshot.PDFTemplateVersion >= 8 {
			digests = []struct{ label, digest string }{{"原始对账单校验码 / SHA-256", statement.PDFSHA256}, {"消费明细校验码 / SHA-256", statement.ManifestSHA256}, {"账单数据校验码 / SHA-256", statement.SnapshotSHA256}}
		}
		if snapshot.PDFTemplateVersion >= 9 {
			label := "配套 Excel / SHA-256"
			if snapshot.PDFTemplateVersion >= 16 {
				label = "配套 Excel 指纹 / SHA-256"
			}
			digests = append(digests, struct{ label, digest string }{label, statement.ExcelSHA256})
		}
		if snapshot.PDFTemplateVersion >= 16 {
			y = doc.verificationInfo(y, statement, digests)
		} else {
			for _, item := range digests {
				y = doc.text(40, y, 515, 8.5, item.label)
				y = doc.text(40, y+6, 515, 8, item.digest) + 16
			}
		}
		receiptNote := "本回执记录客户通过平台真实登录会话执行的确认操作。它不替代电子签章或法定数字签名；原始 PDF 与明细归档保持不变，可通过以上指纹核验关联文件。"
		if snapshot.PDFTemplateVersion >= 8 {
			receiptNote = "本回执记录客户在平台完成的确认操作，不替代电子签章或法定数字签名；可通过以上校验码核对关联文件。"
		}
		y = doc.text(40, y+6, 515, 9, receiptNote)
		if y > 767 {
			return nil, errors.New("billing receipt identity exceeds available page space")
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
