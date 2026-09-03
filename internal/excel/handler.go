package excel

import (
	"fmt"
	"strings"

	"email-verifier-cli/internal/models"
	"github.com/xuri/excelize/v2"
)

type Handler struct {
	File      *excelize.File
	SheetName string
	HeaderMap map[string]int
}

func OpenSpreadsheet(filePath string) (*Handler, []models.Contact, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open Excel file: %w", err)
	}

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		_ = f.Close()
		return nil, nil, fmt.Errorf("file contains no data rows")
	}

	headerMap := make(map[string]int)
	for i, h := range rows[0] {
		headerMap[strings.TrimSpace(h)] = i
	}

	reqCols := []string{"First Name", "Last Name", "Domain", "LinkedIn Profile URL"}
	for _, req := range reqCols {
		if _, ok := headerMap[req]; !ok {
			_ = f.Close()
			return nil, nil, fmt.Errorf("missing required column: '%s'", req)
		}
	}

	outCols := []string{"Verified Email", "Status", "Campaign Sent"}
	for _, col := range outCols {
		if _, exists := headerMap[col]; !exists {
			headerMap[col] = len(headerMap)
			cell, _ := excelize.CoordinatesToCellName(headerMap[col]+1, 1)
			_ = f.SetCellValue(sheetName, cell, col)
		}
	}

	var contacts []models.Contact
	for idx := 1; idx < len(rows); idx++ {
		row := rows[idx]
		getVal := func(colName string) string {
			if cIdx, ok := headerMap[colName]; ok && cIdx < len(row) {
				return strings.TrimSpace(row[cIdx])
			}
			return ""
		}

		contacts = append(contacts, models.Contact{
			RowIndex:      idx + 1,
			FirstName:     getVal("First Name"),
			LastName:      getVal("Last Name"),
			Domain:        getVal("Domain"),
			LinkedInURL:   getVal("LinkedIn Profile URL"),
			VerifiedEmail: getVal("Verified Email"),
			Status:        getVal("Status"),
			CampaignSent:  getVal("Campaign Sent"),
		})
	}

	return &Handler{
		File:      f,
		SheetName: sheetName,
		HeaderMap: headerMap,
	}, contacts, nil
}

func (h *Handler) UpdateRow(rowIndex int, email, status, campaignSent string) error {
	emailCell, _ := excelize.CoordinatesToCellName(h.HeaderMap["Verified Email"]+1, rowIndex)
	statusCell, _ := excelize.CoordinatesToCellName(h.HeaderMap["Status"]+1, rowIndex)
	campaignCell, _ := excelize.CoordinatesToCellName(h.HeaderMap["Campaign Sent"]+1, rowIndex)

	if err := h.File.SetCellValue(h.SheetName, emailCell, email); err != nil {
		return err
	}
	if err := h.File.SetCellValue(h.SheetName, statusCell, status); err != nil {
		return err
	}
	return h.File.SetCellValue(h.SheetName, campaignCell, campaignSent)
}

func (h *Handler) SaveAs(outputPath string) error {
	return h.File.SaveAs(outputPath)
}

func CreateSpreadsheet(outputPath string, contacts []models.Contact) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	headers := []string{"First Name", "Last Name", "Domain", "LinkedIn Profile URL", "Headline", "Verified Email", "Status", "Campaign Sent"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	for idx, c := range contacts {
		row := idx + 2
		vals := []string{c.FirstName, c.LastName, c.Domain, c.LinkedInURL, c.Status, c.VerifiedEmail, c.Status, c.CampaignSent}
		for col, val := range vals {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}

	return f.SaveAs(outputPath)
}

func (h *Handler) Close() error {
	return h.File.Close()
}
