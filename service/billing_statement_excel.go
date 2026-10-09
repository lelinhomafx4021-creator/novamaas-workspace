package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

const billingExcelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// The workbook is another view of the frozen ledger snapshot. It deliberately
// excludes live usage logs, provider costs and inferred historical list prices.
func RenderBillingStatementExcel(statement *model.BillingStatement, snapshot *BillingSnapshot) ([]byte, error) {
	if err := statement.VerifySnapshot(); err != nil {
		return nil, err
	}
	book := excelize.NewFile()
	defer book.Close()
	if err := book.SetSheetName("Sheet1", "总览"); err != nil {
		return nil, err
	}
	for _, name := range []string{"每日汇总", "模型汇总"} {
		if _, err := book.NewSheet(name); err != nil {
			return nil, err
		}
	}
	created := time.Unix(statement.CreatedAt, 0).UTC().Format(time.RFC3339)
	if err := book.SetDocProps(&excelize.DocProperties{Creator: snapshot.Issuer, Title: "月度对账单 " + statement.Month, Created: created, Modified: created, Description: "new-api frozen billing statement"}); err != nil {
		return nil, err
	}
	header, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"19335E"}}, Alignment: &excelize.Alignment{Vertical: "center", WrapText: true}})
	if err != nil {
		return nil, err
	}
	moneyFormat := "#,##0.000000;[Red](#,##0.000000);0.000000"
	if snapshot.PDFTemplateVersion >= 13 {
		symbol := "\"" + strings.ReplaceAll(snapshot.Currency.Symbol, "\"", "\\\"") + "\""
		moneyFormat = `_(` + symbol + `* #,##0.000000_);_(` + symbol + `* (#,##0.000000);_(` + symbol + `* "-"??????_);_(@_)`
	}
	if snapshot.PDFTemplateVersion >= 14 {
		moneyFormat = `_(* #,##0.000000_);_(* (#,##0.000000);_(* "-"??????_);_(@_)`
	}
	moneyStyle := &excelize.Style{CustomNumFmt: &moneyFormat}
	if snapshot.PDFTemplateVersion >= 13 {
		moneyStyle.Alignment = &excelize.Alignment{Horizontal: "right"}
	}
	money, err := book.NewStyle(moneyStyle)
	if err != nil {
		return nil, err
	}
	percent, err := book.NewStyle(&excelize.Style{NumFmt: 10})
	if err != nil {
		return nil, err
	}
	sourceID := statement.SourceStatementID
	if sourceID == "" {
		sourceID = "无（首次生成）"
	}
	rows := [][]any{
		{"月度对账单", "冻结账本金额"},
		{"企业抬头", snapshot.CompanyTitle}, {"税号", snapshot.TaxID},
		{"用户名", snapshot.Username}, {"用户编号", statement.UserID}, {"账期", statement.Month},
		{"运营主体", snapshot.OperatingName}, {"服务平台", snapshot.Issuer},
		{"币种", snapshot.Currency.Code}, {"时区", snapshot.Timezone},
		{"正式记账起点", time.Unix(snapshot.AccountingStartAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")},
		{"本期起点", time.Unix(statement.StartAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")},
		{"本期终点（不含）", time.Unix(statement.EndAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")},
		{"消费金额", billingStatementExcelAmount(snapshot.Total.Charge, snapshot)}, {"退款金额", billingStatementExcelAmount(snapshot.Total.Refund, snapshot)},
		{"本期净额", billingStatementExcelAmount(snapshot.Total.Amount, snapshot)}, {"正式账本记录数", snapshot.Total.Count},
		{"模型数量", billingModelCount(snapshot.Models)}, {"舍入差额", billingStatementExcelAmount(snapshot.RoundingDifference, snapshot)},
		{"计费费率参考时间", time.Unix(snapshot.RateReferenceAt, 0).In(billingLocation).Format("2006-01-02 15:04:05")},
		{"计费费率说明", "生成时用户可用的模型计费组倍率，包含用户专属倍率；多组逐一列示。不是历史实际折扣，不用于倒推原价。"},
		{"原价与令牌说明", "正式账本未保存完整的历史原价、优惠、输入/输出/缓存令牌或成功请求数；未记录不代表零。"},
		{"记录数说明", "消费和退款均为正式入账笔数，不等同于成功请求数；资金充值和临时预扣不计入消费。"},
		{"对账单编号", statement.ID}, {"版本", statement.Revision}, {"来源对账单", sourceID},
	}
	if snapshot.PDFTemplateVersion < 18 {
		rows = append(rows, []any{"账单数据 SHA-256", statement.SnapshotSHA256})
	}
	rows = append(rows,
		[]any{"文件绑定校验", "PDF 列示本 Excel 的 SHA-256；校验清单同时绑定 PDF、Excel 与账单数据。客户确认该清单对应的两个文件。"},
		[]any{"金额精度", "金额保留六位小数；超过 Excel 15 位有效数字的金额以文本保存，避免精度丢失。"},
	)
	if snapshot.PDFTemplateVersion >= 10 {
		rows[19] = []any{"计费口径", "消费发生时的实际计费组与费率"}
		rows[20] = []any{"计费费率说明", "根据历史消费日志或任务提交时的计费快照列示；同一模型按实际计费组与费率拆分。未记录表示缺少可核验历史证据，不使用当前配置替代。"}
		rows[21] = []any{"令牌说明", "正式账本未完整冻结输入、输出及缓存令牌或成功请求数；本表仅列示正式入账明细汇总。"}
	}
	if snapshot.PDFTemplateVersion >= 12 {
		rows = append(rows, []any{"折扣金额说明", billingDiscountAmountNote})
	}
	for i, row := range rows {
		if err := book.SetSheetRow("总览", fmt.Sprintf("A%d", i+1), &row); err != nil {
			return nil, err
		}
	}
	if err := book.SetCellStyle("总览", "A1", "B1", header); err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("总览", "B14", "B16", money); err != nil {
		return nil, err
	}
	if snapshot.PDFTemplateVersion >= 13 {
		if err := book.SetCellStyle("总览", "B19", "B19", money); err != nil {
			return nil, err
		}
	}
	wrap, err := book.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("总览", "B20", fmt.Sprintf("B%d", len(rows)), wrap); err != nil {
		return nil, err
	}
	if err := book.SetColWidth("总览", "A", "A", 27); err != nil {
		return nil, err
	}
	if err := book.SetColWidth("总览", "B", "B", 95); err != nil {
		return nil, err
	}
	noteRows := []int{21, 22, 23, 28, 29}
	if snapshot.PDFTemplateVersion >= 18 {
		noteRows = []int{21, 22, 23, 27, 28}
	}
	for _, row := range noteRows {
		if err := book.SetRowHeight("总览", row, 44); err != nil {
			return nil, err
		}
	}

	if snapshot.PDFTemplateVersion >= 12 {
		if err := book.SetRowHeight("总览", len(rows), 44); err != nil {
			return nil, err
		}
	}

	daily := [][]any{{"日期", "消费金额", "退款金额", "本期净额", "正式账本记录数", "记账范围"}}
	for _, row := range snapshot.Days {
		scope := "正式记账范围"
		if row.State == "outside_period" {
			scope = "正式记账起点之前"
		}
		daily = append(daily, []any{row.Label, billingStatementExcelAmount(row.Charge, snapshot), billingStatementExcelAmount(row.Refund, snapshot), billingStatementExcelAmount(row.Amount, snapshot), row.Count, scope})
	}
	daily = append(daily, []any{"合计", billingStatementExcelAmount(snapshot.Total.Charge, snapshot), billingStatementExcelAmount(snapshot.Total.Refund, snapshot), billingStatementExcelAmount(snapshot.Total.Amount, snapshot), snapshot.Total.Count, ""})
	models := [][]any{{"模型名称", "使用账单天数", "消费笔数", "退款笔数", "正式账本记录数", "消费金额", "退款金额", "本期净额", "计费费率（当前参考）", "标准原价", "消费金额占比", "计费组费率（当前参考）", "首笔入账时间", "末笔入账时间"}}
	if snapshot.PDFTemplateVersion >= 10 {
		models[0] = []any{"模型名称", "使用账单天数", "消费笔数", "退款笔数", "正式账本记录数", "消费金额", "退款金额", "本期净额", "计费费率", "消费金额占比", "实际计费组", "首笔入账时间", "末笔入账时间"}
	}
	for _, row := range snapshot.Models {
		var rate any
		groupLabel := row.BillingGroup
		if snapshot.PDFTemplateVersion >= 10 {
			rate = billingHistoricalRateLabel(row)
			if value, err := decimal.NewFromString(row.BillingRate); err == nil && !value.IsNegative() {
				rate = value.InexactFloat64()
			}
			if groupLabel == "" {
				groupLabel = "未记录"
			}
		} else {
			var groups []string
			for _, reference := range row.CurrentRates {
				value, err := decimal.NewFromString(reference.Ratio)
				if err != nil {
					return nil, err
				}
				groups = append(groups, reference.Group+": "+value.Mul(decimal.NewFromInt(100)).StringFixed(2)+"%")
			}
			groupLabel = strings.Join(groups, "; ")
			rate = billingRateLabel(row)
			if len(row.CurrentRates) == 1 {
				value, err := decimal.NewFromString(row.CurrentRates[0].Ratio)
				if err != nil {
					return nil, err
				}
				rate = value.InexactFloat64()
			}
		}
		share, err := decimal.NewFromString(strings.TrimSuffix(row.ChargeShare, "%"))
		if err != nil {
			return nil, err
		}
		name := row.ModelName
		if name == "" {
			name = "未标注模型"
		}
		values := []any{name, row.ActiveDays, row.ChargeCount, row.RefundCount, row.Count, billingStatementExcelAmount(row.Charge, snapshot), billingStatementExcelAmount(row.Refund, snapshot), billingStatementExcelAmount(row.Amount, snapshot), rate}
		if snapshot.PDFTemplateVersion < 10 {
			values = append(values, "未记录")
		}
		values = append(values, share.Div(decimal.NewFromInt(100)).InexactFloat64(), groupLabel, time.Unix(row.FirstPosted, 0).In(billingLocation).Format("2006-01-02 15:04:05"), time.Unix(row.LastPosted, 0).In(billingLocation).Format("2006-01-02 15:04:05"))
		models = append(models, values)
	}
	var chargeCount, refundCount int64
	for _, row := range snapshot.Models {
		chargeCount += row.ChargeCount
		refundCount += row.RefundCount
	}
	var shareTotal float64
	if snapshot.ChargeQuota > 0 {
		shareTotal = 1
	}
	models = append(models, []any{"合计", "—", chargeCount, refundCount, snapshot.Total.Count, billingStatementExcelAmount(snapshot.Total.Charge, snapshot), billingStatementExcelAmount(snapshot.Total.Refund, snapshot), billingStatementExcelAmount(snapshot.Total.Amount, snapshot), "—", "未记录", shareTotal})
	modelEnd, groupColumn, shareColumn := "N", "L", "K"
	if snapshot.PDFTemplateVersion >= 10 {
		models[len(models)-1] = []any{"合计", "—", chargeCount, refundCount, snapshot.Total.Count, billingStatementExcelAmount(snapshot.Total.Charge, snapshot), billingStatementExcelAmount(snapshot.Total.Refund, snapshot), billingStatementExcelAmount(snapshot.Total.Amount, snapshot), "—", shareTotal}
		modelEnd, groupColumn, shareColumn = "M", "K", "J"
	}
	if snapshot.PDFTemplateVersion >= 18 {
		daily[0][1], daily[0][2], daily[0][3] = "预扣金额", "实际消费金额", "退款金额"
		for i := 1; i < len(daily); i++ {
			daily[i][2], daily[i][3] = daily[i][3], daily[i][2]
		}
		models[0][5], models[0][6], models[0][7] = "预扣金额", "实际消费金额", "退款金额"
		for i := 1; i < len(models); i++ {
			models[i][6], models[i][7] = models[i][7], models[i][6]
		}
	}
	for _, table := range []struct {
		name                      string
		rows                      [][]any
		end, moneyStart, moneyEnd string
	}{
		{"每日汇总", daily, "F", "B", "D"}, {"模型汇总", models, modelEnd, "F", "H"},
	} {
		for i, row := range table.rows {
			if err := book.SetSheetRow(table.name, fmt.Sprintf("A%d", i+1), &row); err != nil {
				return nil, err
			}
		}
		if err := book.SetCellStyle(table.name, "A1", table.end+"1", header); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle(table.name, table.moneyStart+"2", fmt.Sprintf("%s%d", table.moneyEnd, len(table.rows)), money); err != nil {
			return nil, err
		}
		if err := book.SetColWidth(table.name, "A", table.end, 20); err != nil {
			return nil, err
		}
		if err := book.SetColWidth(table.name, "A", "A", 42); err != nil {
			return nil, err
		}
		if snapshot.PDFTemplateVersion >= 13 {
			if err := book.SetColWidth(table.name, table.moneyStart, table.moneyEnd, 30); err != nil {
				return nil, err
			}
		}
		if err := book.SetRowHeight(table.name, 1, 32); err != nil {
			return nil, err
		}
		if err := book.SetPanes(table.name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			return nil, err
		}
	}
	for _, col := range []string{"I", shareColumn} {
		if err := book.SetCellStyle("模型汇总", col+"2", fmt.Sprintf("%s%d", col, len(models)), percent); err != nil {
			return nil, err
		}
	}
	if err := book.SetColWidth("模型汇总", groupColumn, groupColumn, 60); err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("模型汇总", groupColumn+"2", fmt.Sprintf("%s%d", groupColumn, len(models)), wrap); err != nil {
		return nil, err
	}
	for i := 2; i <= len(models); i++ {
		height := 30.0
		if snapshot.PDFTemplateVersion >= 10 {
			height = 45
		}
		if i-2 < len(snapshot.Models) {
			height = max(height, float64(len(snapshot.Models[i-2].CurrentRates))*30)
		}
		if err := book.SetRowHeight("模型汇总", i, min(409, height)); err != nil {
			return nil, err
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	// Excelize iterates package maps. Canonical ZIP ordering and fixed timestamps
	// keep retries byte-identical for immutable OSS objects and PDF fingerprints.
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		return nil, err
	}
	sort.Slice(archive.File, func(i, j int) bool { return archive.File[i].Name < archive.File[j].Name })
	var result bytes.Buffer
	writer := zip.NewWriter(&result)
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: file.Name, Method: zip.Deflate})
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(body); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func billingExcelAmount(value string) any {
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return value
	}
	significant := strings.TrimLeft(strings.ReplaceAll(strings.TrimPrefix(amount.String(), "-"), ".", ""), "0")
	if len(significant) > 15 {
		return value
	}
	return amount.InexactFloat64()
}
