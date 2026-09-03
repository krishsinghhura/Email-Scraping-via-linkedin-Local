package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
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

	header := []string{"First Name", "Last Name", "Domain", "LinkedIn Profile URL", "Headline", "Verified Email", "Status"}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, c := range contacts {
		row := []string{
			c.FirstName,
			c.LastName,
			c.Domain,
			c.LinkedInURL,
			c.Status,
			c.VerifiedEmail,
			"Fetched",
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

	getVal := func(row []string, colName string) string {
		if idx, ok := headerMap[colName]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	var contacts []models.Contact
	for idx := 1; idx < len(records); idx++ {
		row := records[idx]
		contacts = append(contacts, models.Contact{
			RowIndex:      idx + 1,
			FirstName:     getVal(row, "First Name"),
			LastName:      getVal(row, "Last Name"),
			Domain:        getVal(row, "Domain"),
			LinkedInURL:   getVal(row, "LinkedIn Profile URL"),
			Status:        getVal(row, "Headline"),
			VerifiedEmail: getVal(row, "Verified Email"),
		})
	}

	return contacts, nil
}
