package utils

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"github.com/jung-kurt/gofpdf"
)

const (
	pageW   = 210.0
	marginX = 15.0
	contentW = pageW - marginX*2

	labelColW    = 78.0
	valueColW    = contentW - labelColW
	nameColW     = 62.0
	signedColW   = valueColW - nameColW

	lineH      = 4.6
	cellPadY   = 2.2
	cellPadX   = 3.0
	contribRowH = 12.0
)

var (
	colorSectionBar = rgb{186, 216, 240}
	colorFooterNote = rgb{253, 240, 199}
	colorBorder     = rgb{150, 178, 204}
	colorTitle      = rgb{13, 71, 145}
	colorLabel      = rgb{25, 25, 25}
	colorDesc       = rgb{120, 120, 120}
	colorBody       = rgb{40, 40, 40}
)

type rgb struct{ r, g, b int }

type Contributor struct {
	Name       string
	SignedDate string
}

func GenerateRCAPDF(
	report *domain.Report,
	contributors []Contributor,
	documentDate string,
	objectives string,
	kronologi string,
	rootCause string,
	lessonLearnt string,
	tindakan string,
) ([]byte, error) {

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginX, 15, marginX)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	drawHeader(pdf, report.Incident)

	drawSectionBar(pdf, "Document Information", "Information related to this document creation")

	drawContributorsBlock(pdf, contributors)
	drawFieldRow(pdf, "Document Creation Date", "Date of this document first created", documentDate)
	drawFieldRow(pdf, "Objectives", "Objectives of this document", objectives)
	drawFieldRow(pdf, "Scope", "Scope of this document", report.Scope)
	drawFieldRow(pdf, "Severity", "Critical/High/Medium/Low", report.Severity)
	drawFooterNote(pdf, "This approval of this document will use online approval system feature provided by Google Docs")

	pdf.Ln(7)
	drawSectionBar(pdf, "Incident Details", "Explanation of incidents")
	pdf.Ln(3)
	drawParagraph(pdf, "Kronologi Kejadian", kronologi)
	drawParagraph(pdf, "Akar Masalah (Root Cause)", rootCause)

	pdf.Ln(3)
	drawSectionBar(pdf, "Mitigation Details", "How to mitigate the risk, as prevention so it will minimize chances disruption happen in the future")
	pdf.Ln(3)
	drawParagraph(pdf, "Pelajaran yang diambil (Lesson Learnt)", lessonLearnt)
	drawParagraphHeading(pdf, "Tindakan Perbaikan & Pencegahan")
	drawTindakanList(pdf, tindakan)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawHeader(pdf *gofpdf.Fpdf, incidentTitle string) {
	pageCenter := pageW / 2

	pdf.SetFillColor(colorTitle.r, colorTitle.g, colorTitle.b)
	pdf.Circle(pageCenter-24, 20, 4, "F")
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(colorTitle.r, colorTitle.g, colorTitle.b)
	pdf.SetXY(pageCenter-18, 16.5)
	pdf.CellFormat(60, 7, "Management System", "", 0, "L", false, 0, "")

	pdf.SetY(28)
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(contentW, 10, "Root Cause Analysis", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(colorLabel.r, colorLabel.g, colorLabel.b)
	pdf.CellFormat(contentW, 7, fmt.Sprintf("Incident Report: %s", incidentTitle), "", 1, "C", false, 0, "")

	pdf.Ln(6)
}

func drawSectionBar(pdf *gofpdf.Fpdf, title, subtitle string) {
	const (
		topPad    = 2.6
		titleH    = 6.5
		titleGap  = 1.8
		subLineH  = 4.2
		bottomPad = 2.6
	)

	pdf.SetFont("Arial", "I", 8.5)
	var subLines [][]byte
	if subtitle != "" {
		subLines = pdf.SplitLines([]byte(subtitle), contentW-cellPadX*2)
	}

	totalH := topPad + titleH + bottomPad
	if len(subLines) > 0 {
		totalH = topPad + titleH + titleGap + float64(len(subLines))*subLineH + bottomPad
	}

	checkPageBreak(pdf, totalH)
	x, y := pdf.GetX(), pdf.GetY()

	pdf.SetFillColor(colorSectionBar.r, colorSectionBar.g, colorSectionBar.b)
	pdf.SetDrawColor(colorBorder.r, colorBorder.g, colorBorder.b)
	pdf.Rect(x, y, contentW, totalH, "FD")

	pdf.SetXY(x+cellPadX, y+topPad)
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(colorTitle.r, colorTitle.g, colorTitle.b)
	pdf.CellFormat(contentW-cellPadX*2, titleH, title, "", 1, "L", false, 0, "")

	if len(subLines) > 0 {
		pdf.SetXY(x+cellPadX, y+topPad+titleH+titleGap)
		pdf.SetFont("Arial", "I", 8.5)
		pdf.SetTextColor(colorDesc.r, colorDesc.g, colorDesc.b)
		pdf.MultiCell(contentW-cellPadX*2, subLineH, subtitle, "", "L", false)
	}

	pdf.SetXY(x, y+totalH)
}

func drawFieldRow(pdf *gofpdf.Fpdf, label, description, value string) {
	if value == "" {
		value = "(Belum diisi)"
	}

	pdf.SetFont("Arial", "B", 9.5)
	labelLines := pdf.SplitLines([]byte(label), labelColW-cellPadX*2)
	pdf.SetFont("Arial", "I", 8)
	descLines := pdf.SplitLines([]byte(description), labelColW-cellPadX*2)
	pdf.SetFont("Arial", "", 9.5)
	valueLines := pdf.SplitLines([]byte(value), valueColW-cellPadX*2)

	leftLineCount := len(labelLines) + len(descLines)
	rowH := maxFloat(
		float64(leftLineCount)*lineH+cellPadY*2,
		float64(len(valueLines))*lineH+cellPadY*2,
	)
	rowH = maxFloat(rowH, 10)

	checkPageBreak(pdf, rowH)
	x, y := pdf.GetX(), pdf.GetY()

	pdf.SetDrawColor(colorBorder.r, colorBorder.g, colorBorder.b)
	pdf.Rect(x, y, contentW, rowH, "D")
	pdf.Line(x+labelColW, y, x+labelColW, y+rowH)

	pdf.SetXY(x+cellPadX, y+cellPadY)
	pdf.SetFont("Arial", "B", 9.5)
	pdf.SetTextColor(colorLabel.r, colorLabel.g, colorLabel.b)
	pdf.MultiCell(labelColW-cellPadX*2, lineH, label, "", "L", false)

	pdf.SetX(x + cellPadX)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(colorDesc.r, colorDesc.g, colorDesc.b)
	pdf.MultiCell(labelColW-cellPadX*2, lineH, description, "", "L", false)

	pdf.SetXY(x+labelColW+cellPadX, y+cellPadY)
	pdf.SetFont("Arial", "", 9.5)
	pdf.SetTextColor(colorBody.r, colorBody.g, colorBody.b)
	pdf.MultiCell(valueColW-cellPadX*2, lineH, value, "", "L", false)

	pdf.SetXY(x, y+rowH)
}

func drawContributorsBlock(pdf *gofpdf.Fpdf, contributors []Contributor) {
	if len(contributors) == 0 {
		contributors = []Contributor{{Name: "(Belum diisi)"}}
	}

	rowH := contribRowH
	totalH := rowH * float64(len(contributors))

	checkPageBreak(pdf, totalH)
	x, y := pdf.GetX(), pdf.GetY()

	pdf.SetDrawColor(colorBorder.r, colorBorder.g, colorBorder.b)

	pdf.Rect(x, y, contentW, totalH, "D")
	pdf.Line(x+labelColW, y, x+labelColW, y+totalH)
	pdf.Line(x+labelColW+nameColW, y, x+labelColW+nameColW, y+totalH)
	for i := 1; i < len(contributors); i++ {
		ly := y + float64(i)*rowH
		pdf.Line(x+labelColW, ly, x+contentW, ly)
	}

	pdf.SetFont("Arial", "B", 9.5)
	pdf.SetTextColor(colorLabel.r, colorLabel.g, colorLabel.b)
	labelLines := pdf.SplitLines([]byte("Contributors"), labelColW-cellPadX*2)
	pdf.SetFont("Arial", "I", 8)
	descLines := pdf.SplitLines([]byte("Names of those who contributed in the creation of this document"), labelColW-cellPadX*2)
	textBlockH := float64(len(labelLines)+len(descLines)) * lineH
	labelY := y + maxFloat((totalH-textBlockH)/2, cellPadY)

	pdf.SetXY(x+cellPadX, labelY)
	pdf.SetFont("Arial", "B", 9.5)
	pdf.SetTextColor(colorLabel.r, colorLabel.g, colorLabel.b)
	pdf.MultiCell(labelColW-cellPadX*2, lineH, "Contributors", "", "L", false)

	pdf.SetXY(x+cellPadX, pdf.GetY())
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(colorDesc.r, colorDesc.g, colorDesc.b)
	pdf.MultiCell(labelColW-cellPadX*2, lineH, "Names of those who contributed in the creation of this document", "", "L", false)

	pdf.SetFont("Arial", "", 9.5)
	for i, c := range contributors {
		ry := y + float64(i)*rowH

		pdf.SetXY(x+labelColW+cellPadX, ry+cellPadY)
		pdf.SetTextColor(colorBody.r, colorBody.g, colorBody.b)
		pdf.CellFormat(nameColW-cellPadX*2, lineH, c.Name, "", 0, "L", false, 0, "")

		signedText := "Signed"
		dateText := ""
		if c.SignedDate != "" {
			dateText = fmt.Sprintf("(%s)", c.SignedDate)
		} else {
			signedText = "Pending"
		}
		pdf.SetXY(x+labelColW+nameColW+cellPadX, ry+cellPadY)
		pdf.CellFormat(signedColW-cellPadX*2, lineH, signedText, "", 2, "L", false, 0, "")
		pdf.SetX(x + labelColW + nameColW + cellPadX)
		pdf.CellFormat(signedColW-cellPadX*2, lineH, dateText, "", 0, "L", false, 0, "")
	}

	pdf.SetXY(x, y+totalH)
}

func drawFooterNote(pdf *gofpdf.Fpdf, text string) {
	rowH := 9.0
	checkPageBreak(pdf, rowH)
	x, y := pdf.GetX(), pdf.GetY()

	pdf.SetFillColor(colorFooterNote.r, colorFooterNote.g, colorFooterNote.b)
	pdf.SetDrawColor(colorBorder.r, colorBorder.g, colorBorder.b)
	pdf.Rect(x, y, contentW, rowH, "FD")

	pdf.SetXY(x+cellPadX, y+2.2)
	pdf.SetFont("Arial", "I", 8.5)
	pdf.SetTextColor(colorLabel.r, colorLabel.g, colorLabel.b)
	pdf.MultiCell(contentW-cellPadX*2, lineH, text, "", "C", false)

	pdf.SetXY(x, y+rowH)
}

func drawParagraphHeading(pdf *gofpdf.Fpdf, heading string) {
	checkPageBreak(pdf, 8)
	pdf.SetFont("Arial", "B", 10.5)
	pdf.SetTextColor(colorTitle.r, colorTitle.g, colorTitle.b)
	pdf.SetX(marginX)
	pdf.CellFormat(contentW, 6, heading, "", 1, "L", false, 0, "")
}

func drawParagraph(pdf *gofpdf.Fpdf, heading, body string) {
	drawParagraphHeading(pdf, heading)

	pdf.SetFont("Arial", "", 9.5)
	pdf.SetTextColor(colorBody.r, colorBody.g, colorBody.b)
	pdf.SetX(marginX)
	pdf.MultiCell(contentW, lineH, body, "", "L", false)
	pdf.Ln(3)
}

func drawTindakanList(pdf *gofpdf.Fpdf, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	var items []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line != "" {
			items = append(items, line)
		}
	}
	if len(items) == 0 {
		return
	}

	numColW := 8.0
	textColW := contentW - numColW

	pdf.SetX(marginX)
	for i, item := range items {
		lines := pdf.SplitLines([]byte(item), textColW-cellPadX)
		rowH := maxFloat(float64(len(lines))*lineH, lineH)

		checkPageBreak(pdf, rowH)
		x, y := pdf.GetX(), pdf.GetY()

		pdf.SetXY(x, y)
		pdf.SetFont("Arial", "B", 9.5)
		pdf.SetTextColor(colorLabel.r, colorLabel.g, colorLabel.b)
		pdf.CellFormat(numColW, lineH, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")

		pdf.SetXY(x+numColW, y)
		pdf.SetFont("Arial", "", 9.5)
		pdf.SetTextColor(colorBody.r, colorBody.g, colorBody.b)
		pdf.MultiCell(textColW-cellPadX, lineH, item, "", "L", false)

		pdf.SetXY(x, y+rowH)
	}
	pdf.Ln(2)
}

func checkPageBreak(pdf *gofpdf.Fpdf, needed float64) {
	_, pageH := pdf.GetPageSize()
	_, bottomMargin, _, _ := pdf.GetMargins()
	if pdf.GetY()+needed > pageH-bottomMargin {
		pdf.AddPage()
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}