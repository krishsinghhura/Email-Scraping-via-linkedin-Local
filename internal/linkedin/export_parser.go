package linkedin

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"email-verifier-cli/internal/cache"
	"email-verifier-cli/internal/models"
)

func ParseLinkedInExport(filePath string) ([]models.Contact, string, error) {
	filePath = strings.TrimSpace(filePath)
	if strings.HasPrefix(filePath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			filePath = filepath.Join(home, filePath[2:])
		}
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("file not found: %s", filePath)
	}

	suggestedName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	if strings.HasSuffix(strings.ToLower(filePath), ".zip") {
		return parseLinkedInExportZip(filePath)
	}

	if info.IsDir() {
		connPath := filepath.Join(filePath, "Connections.csv")
		if _, err := os.Stat(connPath); err == nil {
			return ParseLinkedInExport(connPath)
		}
		entries, _ := os.ReadDir(filePath)
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".csv") {
				return ParseLinkedInExport(filepath.Join(filePath, e.Name()))
			}
		}
		return nil, suggestedName, fmt.Errorf("no CSV files found in export directory %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, suggestedName, fmt.Errorf("failed to read file: %w", err)
	}

	contacts, err := ParseLinkedInExportCSV(data)
	if err != nil {
		return nil, suggestedName, err
	}

	return contacts, suggestedName, nil
}

func parseLinkedInExportZip(zipPath string) ([]models.Contact, string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer r.Close()

	suggestedName := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	suggestedName = strings.TrimSuffix(suggestedName, ".zip")

	var targetFile *zip.File
	for _, f := range r.File {
		nameLower := strings.ToLower(f.Name)
		if strings.Contains(nameLower, "connection") && strings.HasSuffix(nameLower, ".csv") {
			targetFile = f
			break
		}
	}

	if targetFile == nil {
		for _, f := range r.File {
			if strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
				targetFile = f
				break
			}
		}
	}

	if targetFile == nil {
		return nil, suggestedName, fmt.Errorf("no CSV files found inside zip archive")
	}

	rc, err := targetFile.Open()
	if err != nil {
		return nil, suggestedName, fmt.Errorf("failed to read file inside zip: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, suggestedName, fmt.Errorf("failed to extract file from zip: %w", err)
	}

	contacts, err := ParseLinkedInExportCSV(data)
	if err != nil {
		return nil, suggestedName, err
	}

	return contacts, suggestedName, nil
}

func ParseLinkedInExportCSV(data []byte) ([]models.Contact, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	headerRowIdx := -1
	headerMap := make(map[string]int)

	for idx, row := range records {
		for _, col := range row {
			cleaned := strings.TrimSpace(strings.ToLower(col))
			if cleaned == "first name" || cleaned == "firstname" {
				headerRowIdx = idx
				break
			}
		}
		if headerRowIdx != -1 {
			for colIdx, col := range row {
				headerMap[strings.TrimSpace(strings.ToLower(col))] = colIdx
			}
			break
		}
	}

	if headerRowIdx == -1 {
		return nil, fmt.Errorf("could not find LinkedIn export header row")
	}

	getVal := func(row []string, colNames ...string) string {
		for _, name := range colNames {
			if idx, ok := headerMap[strings.ToLower(name)]; ok && idx < len(row) {
				val := strings.TrimSpace(row[idx])
				if val != "" {
					return val
				}
			}
		}
		return ""
	}

	var contacts []models.Contact
	seen := make(map[string]bool)

	for i := headerRowIdx + 1; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 {
			continue
		}

		fn := getVal(row, "first name", "firstname")
		ln := getVal(row, "last name", "lastname")
		urlStr := getVal(row, "url", "linkedin profile url", "profile url")
		email := getVal(row, "email address", "email", "verified email")
		company := getVal(row, "company", "current company")
		position := getVal(row, "position", "headline", "job title")

		if fn == "" && ln == "" && urlStr == "" {
			continue
		}

		key := urlStr
		if key == "" {
			key = fmt.Sprintf("%s %s", fn, ln)
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		headline := position
		if company != "" {
			if headline != "" {
				headline = fmt.Sprintf("%s at %s", headline, company)
			} else {
				headline = company
			}
		}

		dom := ""
		if company != "" {
			if cached, ok := cache.GetGlobalStore().GetCompanyDomain(company); ok && cached != "" {
				dom = cached
			}
		}

		contacts = append(contacts, models.Contact{
			RowIndex:        len(contacts) + 1,
			FirstName:       fn,
			LastName:        ln,
			Domain:          dom,
			LinkedInURL:     urlStr,
			RegisteredEmail: email,
			VerifiedEmail:   email,
			Status:          headline,
		})
	}

	return contacts, nil
}
