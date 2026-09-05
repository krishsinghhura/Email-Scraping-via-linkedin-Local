package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"email-verifier-cli/internal/models"
)

func SaveConnectionsToCSV(filename string, contacts []models.Contact) (string, error) {
	if !strings.HasSuffix(filename, ".csv") {
		filename = filename + ".csv"
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}

	file, err := os.Create(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to create CSV file '%s': %w", absPath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
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
	}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, c := range contacts {
		scoreStr := fmt.Sprintf("%d%%", c.ConfidenceScore)

		row := []string{
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
		}
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return absPath, nil
}

func ReadContactsFromCSV(filePath string) ([]models.Contact, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file '%s': %w", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		return nil, fmt.Errorf("CSV file contains no valid data rows")
	}

	headerMap := make(map[string]int)
	for i, h := range records[0] {
		headerMap[strings.TrimSpace(h)] = i
	}

	getVal := func(row []string, colNames ...string) string {
		for _, name := range colNames {
			if idx, ok := headerMap[name]; ok && idx < len(row) {
				v := strings.TrimSpace(row[idx])
				if v != "" {
					return v
				}
			}
		}
		return ""
	}

	var contacts []models.Contact
	for idx := 1; idx < len(records); idx++ {
		row := records[idx]

		scoreVal := 0
		if rawScore := getVal(row, "Deliverability Score", "Confidence Score"); rawScore != "" {
			cleanScore := strings.TrimSuffix(rawScore, "%")
			if s, err := strconv.Atoi(cleanScore); err == nil {
				scoreVal = s
			}
		}

		contacts = append(contacts, models.Contact{
			RowIndex:           idx + 1,
			FirstName:          getVal(row, "First Name"),
			LastName:           getVal(row, "Last Name"),
			Domain:             getVal(row, "Company Domain", "Domain"),
			PersonalEmail:      getVal(row, "Personal Email"),
			WorkEmail:          getVal(row, "Work Email"),
			VerifiedEmail:      getVal(row, "Verified Email"),
			ConfidenceScore:    scoreVal,
			ConfidenceTier:     getVal(row, "Confidence Tier"),
			VerificationReason: getVal(row, "Verification Reason"),
			LinkedInURL:        getVal(row, "LinkedIn Profile URL"),
			Status:             getVal(row, "Headline", "Status"),
		})
	}

	return contacts, nil
}
