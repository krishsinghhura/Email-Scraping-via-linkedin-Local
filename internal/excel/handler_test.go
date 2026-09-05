package excel

import (
	"os"
	"path/filepath"
	"testing"

	"email-verifier-cli/internal/models"
)

func TestCreateAndOpenSpreadsheetWithScoring(t *testing.T) {
	tempDir := t.TempDir()
	xlsxPath := filepath.Join(tempDir, "test_output.xlsx")

	contacts := []models.Contact{
		{
			RowIndex:           2,
			FirstName:          "Alice",
			LastName:           "Wonderland",
			Domain:             "wonder.org",
			PersonalEmail:      "alice@gmail.com",
			WorkEmail:          "alice@wonder.org",
			VerifiedEmail:      "alice@gmail.com",
			ConfidenceScore:    100,
			ConfidenceTier:     "Safe to Send",
			VerificationReason: "SMTP 250 OK (Personal Confirmed)",
			Status:             "Verified (Personal)",
		},
		{
			RowIndex:           3,
			FirstName:          "Bob",
			LastName:           "Builder",
			Domain:             "build.co",
			WorkEmail:          "bob.builder@build.co",
			VerifiedEmail:      "bob.builder@build.co",
			ConfidenceScore:    75,
			ConfidenceTier:     "Accept-All (Risky)",
			VerificationReason: "Catch-All Server (Verified Company Pattern Applied)",
			Status:             "Catch-All (High Confidence)",
		},
	}

	if err := CreateSpreadsheet(xlsxPath, contacts); err != nil {
		t.Fatalf("CreateSpreadsheet failed: %v", err)
	}

	handler, loaded, err := OpenSpreadsheet(xlsxPath)
	if err != nil {
		t.Fatalf("OpenSpreadsheet failed: %v", err)
	}
	defer handler.Close()

	if len(loaded) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(loaded))
	}

	if loaded[0].ConfidenceScore != 100 || loaded[0].ConfidenceTier != "Safe to Send" {
		t.Errorf("loaded[0] score mismatch: %+v", loaded[0])
	}
	if loaded[1].ConfidenceScore != 75 || loaded[1].ConfidenceTier != "Accept-All (Risky)" {
		t.Errorf("loaded[1] score mismatch: %+v", loaded[1])
	}

	// Test UpdateContactRow
	loaded[1].ConfidenceScore = 95
	loaded[1].ConfidenceTier = "Safe to Send"
	loaded[1].VerificationReason = "VRFY Confirmed"
	if err := handler.UpdateContactRow(loaded[1]); err != nil {
		t.Fatalf("UpdateContactRow failed: %v", err)
	}

	_ = os.Remove(xlsxPath)
}
