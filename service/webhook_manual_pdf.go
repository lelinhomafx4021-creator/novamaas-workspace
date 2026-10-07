package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"github.com/signintech/gopdf"
)

type webhookManualSection struct {
	Title string
	Body  []string
	Rows  [][3]string
	Code  string
}

type webhookManualPage struct {
	Title    string
	Sections []webhookManualSection
}

type webhookManualLayoutItem struct {
	Title   string
	Section *webhookManualSection
}

// RenderWebhookManualPDF uses the current document branding, the same embedded
// Chinese font, A4 margins, letterhead colours and page footer as system PDFs.
// It never includes actual callback addresses, customer identities or tokens.
func RenderWebhookManualPDF(ctx context.Context, scope WebhookManualScope) ([]byte, error) {
	if !scope.Valid() {
		return nil, errors.New("invalid webhook manual scope")
	}
	branding, err := captureBillingDocumentBranding(ctx)
	if err != nil {
		return nil, err
	}
	return renderWebhookManualPDF(branding, scope)
}

func renderWebhookManualPDF(branding *billingDocumentBranding, scope WebhookManualScope) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.SetInfo(gopdf.PdfInfo{Title: scope.Label() + "钩子接口手册 / Webhook API Manual", Author: branding.Issuer, Creator: "new-api / QuantumNous webhook API", CreationDate: time.Now().UTC()})
	missingGlyph := false
	if err := pdf.AddTTFFontDataWithOption("billing", billingPDFFont, gopdf.TtfOption{OnGlyphNotFound: func(r rune) { missingGlyph = true }, OnGlyphNotFoundSubstitute: gopdf.DefaultOnGlyphNotFoundSubstitute}); err != nil {
		return nil, err
	}
	pages, err := planWebhookManualPages(pdf, webhookManualPages(scope))
	if err != nil {
		return nil, err
	}
	doc := &billingPDFDocument{pdf: pdf}
	for index, page := range pages {
		doc.webhookManualHeader(branding, scope, index+1, len(pages))
		if doc.err != nil {
			return nil, doc.err
		}
		y := 120.0
		for _, item := range page {
			if item.Title != "" {
				pdf.SetTextColor(25, 51, 94)
				y = doc.text(40, y, 515, 22, item.Title) + 18
				continue
			}
			section := item.Section
			if section.Title != "" {
				pdf.SetTextColor(25, 51, 94)
				y = doc.text(40, y, 515, 12, section.Title) + 8
			}
			pdf.SetTextColor(28, 43, 64)
			for _, body := range section.Body {
				y = doc.text(40, y, 515, 9.3, body) + 8
			}
			if len(section.Rows) > 0 {
				y = doc.webhookManualTable(y, section.Rows) + 12
			}
			if section.Code != "" {
				y = doc.webhookManualCode(y+6, section.Code) + 12
			}
			if doc.err != nil {
				return nil, doc.err
			}
			if y > 770 {
				return nil, fmt.Errorf("webhook manual page %d exceeds content area", index+1)
			}
		}
	}
	if doc.err != nil {
		return nil, doc.err
	}
	if missingGlyph {
		return nil, errors.New("webhook manual branding contains unsupported font characters")
	}
	return pdf.GetBytesPdfReturnErr()
}

// Plan complete sections before drawing, keeping headings with their first
// section and avoiding the large blank areas from one chapter per A4 sheet.
func planWebhookManualPages(pdf *gopdf.GoPdf, chapters []webhookManualPage) ([][]webhookManualLayoutItem, error) {
	pages := [][]webhookManualLayoutItem{{}}
	y := 120.0
	for _, chapter := range chapters {
		titleHeight, err := webhookManualTextHeight(pdf, 22, 515, chapter.Title)
		if err != nil {
			return nil, err
		}
		titleHeight += 18
		if len(chapter.Sections) == 0 {
			continue
		}
		firstHeight, err := webhookManualSectionHeight(pdf, chapter.Sections[0])
		if err != nil {
			return nil, err
		}
		if y+titleHeight+firstHeight > 770 {
			pages = append(pages, nil)
			y = 120
		}
		pages[len(pages)-1] = append(pages[len(pages)-1], webhookManualLayoutItem{Title: chapter.Title})
		y += titleHeight
		for _, section := range chapter.Sections {
			height, err := webhookManualSectionHeight(pdf, section)
			if err != nil {
				return nil, err
			}
			if y+height > 770 {
				pages = append(pages, nil)
				y = 120
				continuation := chapter.Title + "（续）"
				continuationHeight, err := webhookManualTextHeight(pdf, 22, 515, continuation)
				if err != nil {
					return nil, err
				}
				pages[len(pages)-1] = append(pages[len(pages)-1], webhookManualLayoutItem{Title: continuation})
				y += continuationHeight + 18
			}
			if y+height > 770 {
				return nil, errors.New("webhook manual section exceeds content area")
			}
			sectionCopy := section
			pages[len(pages)-1] = append(pages[len(pages)-1], webhookManualLayoutItem{Section: &sectionCopy})
			y += height
		}
	}
	return pages, nil
}

func webhookManualTextHeight(pdf *gopdf.GoPdf, size, width float64, value string) (float64, error) {
	if err := pdf.SetFont("billing", "", size); err != nil {
		return 0, err
	}
	lines, err := pdf.SplitText(value, width)
	if err != nil {
		return 0, err
	}
	return float64(len(lines)) * size * 1.45, nil
}

func webhookManualSectionHeight(pdf *gopdf.GoPdf, section webhookManualSection) (float64, error) {
	height := 0.0
	if section.Title != "" {
		textHeight, err := webhookManualTextHeight(pdf, 12, 515, section.Title)
		if err != nil {
			return 0, err
		}
		height += textHeight + 8
	}
	for _, body := range section.Body {
		textHeight, err := webhookManualTextHeight(pdf, 9.3, 515, body)
		if err != nil {
			return 0, err
		}
		height += textHeight + 8
	}
	if len(section.Rows) > 0 {
		for _, row := range append([][3]string{{"参数 / 路径", "类型 / 方法", "说明"}}, section.Rows...) {
			rowHeight := 22.0
			for column, value := range row {
				textHeight, err := webhookManualTextHeight(pdf, 8.5, []float64{158, 54, 261}[column], value)
				if err != nil {
					return 0, err
				}
				rowHeight = math.Max(rowHeight, textHeight+10)
			}
			height += rowHeight
		}
		height += 12
	}
	if section.Code != "" {
		height += 6 + 16 + float64(len(strings.Split(section.Code, "\n")))*12 + 12
	}
	return height, nil
}

func (doc *billingPDFDocument) webhookManualHeader(branding *billingDocumentBranding, scope WebhookManualScope, page, pages int) {
	doc.pdf.AddPage()
	for _, mark := range []struct {
		Body       []byte
		X, Y, W, H float64
	}{
		{branding.LogoPNG, 40, 33, 38, 38}, {branding.OperatingLogoPNG, 430, 33, 125, 40},
	} {
		if len(mark.Body) == 0 {
			continue
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(mark.Body))
		if err != nil {
			doc.err = err
			return
		}
		holder, err := gopdf.ImageHolderByBytes(mark.Body)
		if err != nil {
			doc.err = err
			return
		}
		scale := math.Min(mark.W/float64(config.Width), mark.H/float64(config.Height))
		width, height := float64(config.Width)*scale, float64(config.Height)*scale
		x := mark.X + (mark.W-width)/2
		if mark.X == 430 {
			x = 555 - width
		}
		if doc.err = doc.pdf.ImageByHolder(holder, x, mark.Y+(mark.H-height)/2, &gopdf.Rect{W: width, H: height}); doc.err != nil {
			return
		}
	}
	doc.pdf.SetTextColor(25, 51, 94)
	width := 465.0
	if branding.OperatingName != "" || len(branding.OperatingLogoPNG) > 0 {
		width = 325
	}
	end := doc.text(90, 34, width, 15, branding.Issuer)
	if end > 76 {
		doc.err = errors.New("platform name exceeds manual letterhead")
		return
	}
	doc.pdf.SetTextColor(90, 106, 129)
	doc.text(90, end+1, width, 7.5, "WEBHOOK API / "+scope.Label())
	if branding.OperatingName != "" {
		doc.right(430, 79, 125, 8, branding.OperatingName)
	}
	if doc.err != nil {
		return
	}
	doc.pdf.SetStrokeColor(39, 71, 122)
	doc.pdf.SetLineWidth(1.5)
	doc.pdf.Line(40, 100, 555, 100)
	doc.pdf.SetStrokeColor(221, 228, 237)
	doc.pdf.SetLineWidth(0.5)
	doc.pdf.Line(40, 784, 555, 784)
	footer := branding.Footer
	if footer == "" {
		footer = branding.Issuer + " | Webhook API | new-api / QuantumNous"
	}
	fitted := false
	for _, size := range []float64{7, 6.4, 6} {
		if doc.err = doc.pdf.SetFont("billing", "", size); doc.err != nil {
			return
		}
		lines, err := doc.pdf.SplitText(footer, 445)
		if err != nil {
			doc.err = err
			return
		}
		if 793+float64(len(lines)-1)*size*1.45+size <= 820 {
			doc.text(40, 793, 445, size, footer)
			fitted = true
			break
		}
	}
	if !fitted {
		doc.err = errors.New("webhook manual footer exceeds available page space")
		return
	}
	doc.right(500, 794, 55, 8, fmt.Sprintf("%02d / %02d", page, pages))
}

func (doc *billingPDFDocument) webhookManualCode(y float64, code string) float64 {
	if doc.err != nil {
		return y
	}
	lines := strings.Split(code, "\n")
	height := 16 + float64(len(lines))*12
	if y+height > 770 {
		doc.err = errors.New("webhook manual JSON example exceeds content area")
		return y
	}
	if doc.err = doc.pdf.SetFont("billing", "", 8.2); doc.err != nil {
		return y
	}
	for _, line := range lines {
		wrapped, err := doc.pdf.SplitText(line, 485)
		if err != nil {
			doc.err = err
			return y
		}
		if len(wrapped) > 1 {
			doc.err = errors.New("webhook manual JSON line exceeds code width")
			return y
		}
	}
	doc.pdf.SetFillColor(244, 247, 252)
	doc.pdf.RectFromUpperLeftWithStyle(40, y, 515, height, "F")
	doc.pdf.SetFillColor(39, 71, 122)
	doc.pdf.RectFromUpperLeftWithStyle(40, y, 3, height, "F")
	doc.pdf.SetTextColor(39, 71, 122)
	for index, line := range lines {
		doc.text(51, y+8+float64(index)*12, 484, 8.2, line)
		if doc.err != nil {
			return y
		}
	}
	doc.pdf.SetTextColor(28, 43, 64)
	return y + height
}

func (doc *billingPDFDocument) webhookManualTable(y float64, rows [][3]string) float64 {
	if doc.err != nil {
		return y
	}
	widths := []float64{172, 68, 275}
	for index, row := range append([][3]string{{"参数 / 路径", "类型 / 方法", "说明"}}, rows...) {
		height := 22.0
		if doc.err = doc.pdf.SetFont("billing", "", 8.5); doc.err != nil {
			return y
		}
		for column, value := range row {
			lines, err := doc.pdf.SplitText(value, widths[column]-14)
			if err != nil {
				doc.err = err
				return y
			}
			height = math.Max(height, float64(len(lines))*8.5*1.45+10)
		}
		if y+height > 770 {
			doc.err = errors.New("webhook manual table exceeds content area")
			return y
		}
		if index == 0 {
			doc.pdf.SetFillColor(234, 242, 252)
			doc.pdf.RectFromUpperLeftWithStyle(40, y, 515, height, "F")
			doc.pdf.SetTextColor(25, 51, 94)
		} else {
			doc.pdf.SetTextColor(28, 43, 64)
		}
		x := 40.0
		for column, value := range row {
			doc.text(x+7, y+5, widths[column]-14, 8.5, value)
			if doc.err != nil {
				return y
			}
			x += widths[column]
		}
		doc.pdf.SetStrokeColor(221, 228, 237)
		doc.pdf.SetLineWidth(0.5)
		doc.pdf.Line(40, y+height, 555, y+height)
		y += height
	}
	return y
}
