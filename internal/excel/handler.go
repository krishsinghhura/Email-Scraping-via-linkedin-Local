package excel

import (
	"fmt"
	"strconv"
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

	reqCols := []string{"First Name", "Last Name"}
	for _, req := range reqCols {
		if _, ok := headerMap[req]; !ok {
			_ = f.Close()
			return nil, nil, fmt.Errorf("missing required column: '%s'", req)
		}
	}

	outCols := []string{
		"Personal Email", "Work Email", "Verified Email",
		"Deliverability Score", "Confidence Tier", "Verification Reason",
		"Status", "Campaign Sent",
	}
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
		getVal := func(colNames ...string) string {
			for _, colName := range colNames {
				if cIdx, ok := headerMap[colName]; ok && cIdx < len(row) {
					v := strings.TrimSpace(row[cIdx])
					if v != "" {
						return v
					}
				}
			}
			return ""
		}

		scoreVal := 0
		if rawScore := getVal("Deliverability Score", "Confidence Score"); rawScore != "" {
			cleanScore := strings.TrimSuffix(rawScore, "%")
			if s, err := strconv.Atoi(cleanScore); err == nil {
				scoreVal = s
			}
		}

		contacts = append(contacts, models.Contact{
			RowIndex:           idx + 1,
			FirstName:          getVal("First Name"),
			LastName:           getVal("Last Name"),
			Domain:             getVal("Company Domain", "Domain"),
			LinkedInURL:        getVal("LinkedIn Profile URL", "LinkedIn URL"),
			PersonalEmail:      getVal("Personal Email"),
			WorkEmail:          getVal("Work Email"),
			VerifiedEmail:      getVal("Verified Email"),
			ConfidenceScore:    scoreVal,
			ConfidenceTier:     getVal("Confidence Tier"),
			VerificationReason: getVal("Verification Reason"),
			Status:             getVal("Headline", "Status"),
			CampaignSent:       getVal("Campaign Sent"),
		})
	}

	return &Handler{
		File:      f,
		SheetName: sheetName,
		HeaderMap: headerMap,
	}, contacts, nil
}

func (h *Handler) UpdateContactRow(c models.Contact) error {
	scoreStr := fmt.Sprintf("%d%%", c.ConfidenceScore)

	if idx, ok := h.HeaderMap["Personal Email"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, c.PersonalEmail)
	}
	if idx, ok := h.HeaderMap["Work Email"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, c.WorkEmail)
	}
	if idx, ok := h.HeaderMap["Verified Email"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, c.VerifiedEmail)
	}
	if idx, ok := h.HeaderMap["Deliverability Score"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, scoreStr)
	}
	if idx, ok := h.HeaderMap["Confidence Tier"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, c.ConfidenceTier)
	}
	if idx, ok := h.HeaderMap["Verification Reason"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, c.VerificationReason)
	}
	if idx, ok := h.HeaderMap["Status"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, c.Status)
	}
	if idx, ok := h.HeaderMap["Campaign Sent"]; ok {
		cell, _ := excelize.CoordinatesToCellName(idx+1, c.RowIndex)
		_ = h.File.SetCellValue(h.SheetName, cell, c.CampaignSent)
	}
	return nil
}

func (h *Handler) UpdateRow(rowIndex int, personalEmail, workEmail, verifiedEmail, status, campaignSent string) error {
	return h.UpdateContactRow(models.Contact{
		RowIndex:      rowIndex,
		PersonalEmail: personalEmail,
		WorkEmail:     workEmail,
		VerifiedEmail: verifiedEmail,
		Status:        status,
		CampaignSent:  campaignSent,
	})
}

func (h *Handler) SaveAs(outputPath string) error {
	return h.File.SaveAs(outputPath)
}

func CreateSpreadsheet(outputPath string, contacts []models.Contact) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	headers := []string{
		"First Name",
		"Last Name",
		"Company Domain",
		"Personal Email",
		"Work Email",
		"Verified Email",
		"Deliverability Score",
		"Confidence Tier",
		"Verification Reason",
		"Status",
		"LinkedIn Profile URL",
		"Headline",
		"Campaign Sent",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	for idx, c := range contacts {
		row := idx + 2
		scoreStr := fmt.Sprintf("%d%%", c.ConfidenceScore)

		vals := []string{
			c.FirstName,
			c.LastName,
			c.Domain,
			c.PersonalEmail,
			c.WorkEmail,
			c.VerifiedEmail,
			scoreStr,
			c.ConfidenceTier,
			c.VerificationReason,
			c.Status,
			c.LinkedInURL,
			c.Status,
			c.CampaignSent,
		}
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
