package service

import (
	"errors"
	"strings"

	"github.com/shopspring/decimal"
)

// Accounting amounts share exact decimal formatting across PDF and text cells
// that exceed Excel's 15 significant digit precision.
func billingAccountingNumber(value string) (string, error) {
	return billingDisplayAmount(value, 6, true)
}

func billingDisplayAmount(value string, decimalPlaces int32, zeroDash bool) (string, error) {
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return "", err
	}
	fixed := amount.Abs().StringFixed(decimalPlaces)
	if amount.Round(decimalPlaces).IsZero() {
		if zeroDash {
			return "-", nil
		}
		return fixed, nil
	}
	parts := strings.SplitN(fixed, ".", 2)
	integer := parts[0]
	var grouped strings.Builder
	for i, digit := range integer {
		if i > 0 && (len(integer)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	grouped.WriteByte('.')
	grouped.WriteString(parts[1])
	result := grouped.String()
	if amount.IsNegative() {
		result = "(" + result + ")"
	}
	return result, nil
}

// Postpaid balances retain exact decimals until the document formats them.
// Consumption reduces the balance; a net refund increases it.
func billingPostpaidDailyBalanceAmounts(days []BillingRow) ([]string, error) {
	balances := make([]string, len(days))
	consumption := decimal.Zero
	for i, day := range days {
		if day.State == "outside_period" {
			balances[i] = "-"
			continue
		}
		amount, err := decimal.NewFromString(day.Amount)
		if err != nil {
			return nil, err
		}
		consumption = consumption.Add(amount)
		balances[i] = consumption.Neg().String()
	}
	return balances, nil
}

// Preserve the signed display used by archived templates through version 17.
func billingPostpaidDailyBalances(days []BillingRow) ([]string, error) {
	balances, err := billingPostpaidDailyBalanceAmounts(days)
	if err != nil {
		return nil, err
	}
	for i, balance := range balances {
		if balance == "-" {
			continue
		}
		number, err := billingDisplayAmount(balance, 2, false)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(number, "(") {
			number = "-" + strings.TrimSuffix(strings.TrimPrefix(number, "("), ")")
		}
		balances[i] = number
	}
	return balances, nil
}

// Align two-place accounting amounts, including negatives, at the right edge.
func (doc *billingPDFDocument) accounting(x, y, width, size float64, value string) {
	if doc.err != nil {
		return
	}
	number, err := billingDisplayAmount(value, 2, false)
	if err != nil {
		doc.err = err
		return
	}
	for size >= 6 {
		if doc.err = doc.pdf.SetFont("billing", "", size); doc.err != nil {
			return
		}
		numberWidth, err := doc.pdf.MeasureTextWidth(number)
		if err != nil {
			doc.err = err
			return
		}
		padding := ")"
		if strings.HasPrefix(number, "(") {
			padding = ""
		}
		rightInset, err := doc.pdf.MeasureTextWidth(padding)
		if err != nil {
			doc.err = err
			return
		}
		if numberWidth+rightInset <= width {
			doc.right(x, y, width-rightInset, size, number)
			return
		}
		size -= 0.5
	}
	doc.err = errors.New("billing accounting amount exceeds PDF column width")
}

func billingStatementExcelAmount(value string, snapshot *BillingSnapshot) any {
	cell := billingExcelAmount(value)
	if snapshot.PDFTemplateVersion < 13 {
		return cell
	}
	if _, isText := cell.(string); !isText {
		return cell
	}
	number, err := billingAccountingNumber(value)
	if err != nil {
		return cell
	}
	if snapshot.PDFTemplateVersion >= 14 {
		return number
	}
	return snapshot.Currency.Symbol + " " + number
}
