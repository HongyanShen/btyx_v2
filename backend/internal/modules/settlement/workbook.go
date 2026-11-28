package settlement

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"github.com/shopspring/decimal"
)

// Workbook exports only posted lines. Inline text keeps amounts and IDs exact in Excel.
func Workbook(week WeekDTO, profiles []identity.UserDTO) ([]byte, error) {
	names := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		name := profile.Name
		if name == "" {
			name = profile.Nickname
		}

		names[profile.ID] = name
	}

	type postedTotal struct {
		id          string
		assignments map[string]bool
		amount      decimal.Decimal
	}

	indexes := make(map[string]int)
	var totals []postedTotal
	for _, line := range week.Lines {
		if line.CreditedAt == nil {
			continue
		}

		index, exists := indexes[line.UserID]
		if !exists {
			index = len(totals)
			indexes[line.UserID] = index
			totals = append(totals, postedTotal{
				id:          line.UserID,
				assignments: make(map[string]bool),
			})
		}

		amount, err := decimal.NewFromString(line.AmountFen)
		if err != nil {
			return nil, err
		}

		totals[index].assignments[line.AssignmentID] = true
		totals[index].amount = totals[index].amount.Add(amount)
	}

	var sheet bytes.Buffer
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	if err := writeSheetRow(&sheet, "分发员", "分发员编号", "条数", "金额（元）", "结算日期"); err != nil {
		return nil, err
	}

	for _, total := range totals {
		name := names[total.id]
		if name == "" {
			name = "分发员"
		}

		if err := writeSheetRow(&sheet, name, total.id, strconv.Itoa(len(total.assignments)), total.amount.Shift(-2).StringFixed(2), week.Through); err != nil {
			return nil, err
		}
	}

	sheet.WriteString("</sheetData></worksheet>")

	var file bytes.Buffer
	writer := zip.NewWriter(&file)
	const relationships = "http://schemas.openxmlformats.org/package/2006/relationships"
	const document = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	parts := []struct {
		path string
		body string
	}{
		{
			path: "[Content_Types].xml",
			body: `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`,
		},
		{
			path: "_rels/.rels",
			body: fmt.Sprintf(`<Relationships xmlns="%s"><Relationship Id="rId1" Type="%s/officeDocument" Target="xl/workbook.xml"/></Relationships>`, relationships, document),
		},
		{
			path: "xl/workbook.xml",
			body: fmt.Sprintf(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="%s"><sheets><sheet name="入账明细" sheetId="1" r:id="rId1"/></sheets></workbook>`, document),
		},
		{
			path: "xl/_rels/workbook.xml.rels",
			body: fmt.Sprintf(`<Relationships xmlns="%s"><Relationship Id="rId1" Type="%s/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`, relationships, document),
		},
		{
			path: "xl/worksheets/sheet1.xml",
			body: sheet.String(),
		},
	}
	for _, part := range parts {
		entry, err := writer.Create(part.path)
		if err != nil {
			return nil, err
		}

		if _, err := io.WriteString(entry, part.body); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return file.Bytes(), nil
}

func writeSheetRow(sheet *bytes.Buffer, values ...string) error {
	sheet.WriteString("<row>")
	for _, value := range values {
		sheet.WriteString(`<c t="inlineStr"><is><t xml:space="preserve">`)
		if err := xml.EscapeText(sheet, []byte(value)); err != nil {
			return err
		}

		sheet.WriteString("</t></is></c>")
	}

	sheet.WriteString("</row>")
	return nil
}
