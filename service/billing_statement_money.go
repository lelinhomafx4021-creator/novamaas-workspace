package service

import (
	"errors"
	"strings"

	"github.com/shopspring/decimal"
)

// Accounting amounts share exact decimal formatting across PDF and text cells
// that exceed Excel's 15 significant digit precision.
func billingAccountingNumber(value string) (string, error) {
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return "", err
	}
	fixed := amount.Abs().StringFixed(6)
	if fixed == "0.000000" {
		return "-", nil
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

// Decimal places align at the right edge, including negative amounts and
// accounting zero dashes. Legacy templates may also include a currency symbol.
func (doc *billingPDFDocument) accounting(x, y, width, size float64, value, symbol string) {
	if doc.err != nil {
		return
	}
	number, err := billingAccountingNumber(value)
	if err != nil {
		doc.err = err
		return
	}
	for size >= 6 {
		if doc.err = doc.pdf.SetFont("billing", "", size); doc.err != nil {
			return
		}
		symbolWidth, err := doc.pdf.MeasureTextWidth(symbol)
		if err != nil {
			doc.err = err
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
		} else if number == "-" {
			padding = ".000000)"
		}
		rightInset, err := doc.pdf.MeasureTextWidth(padding)
		if err != nil {
			doc.err = err
			return
		}
		leftInset := 0.0
		if symbol != "" {
			leftInset = symbolWidth + 3
		}
		if leftInset+numberWidth+rightInset <= width {
			if symbol != "" {
				doc.pdf.SetXY(x, y)
				if doc.err = doc.pdf.Cell(nil, symbol); doc.err != nil {
					return
				}
			}
			doc.right(x+leftInset, y, width-leftInset-rightInset, size, number)
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
