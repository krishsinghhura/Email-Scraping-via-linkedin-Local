package csv

import (
	"os"
	"path/filepath"
	"testing"

	"email-verifier-cli/internal/models"
)

func TestSaveAndReadConnectionsCSV(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test_connections.csv")

	contacts := []models.Contact{
		{
			FirstName:          "Jane",
			LastName:           "Doe",
			Domain:             "example.com",
			LinkedInURL:        "https://www.linkedin.com/in/jane-doe",
			PersonalEmail:      "jane.doe@gmail.com",
			WorkEmail:          "jane.doe@example.com",
			VerifiedEmail:      "jane.doe@gmail.com",
			ConfidenceScore:    100,
			ConfidenceTier:     "Safe to Send",
			VerificationReason: "SMTP 250 OK (Personal & Corporate Confirmed)",
			Status:             "Senior Product Manager at ExampleCorp",
		},
		{
			FirstName:          "John",
			LastName:           "Smith",
			Domain:             "acme.com",
			LinkedInURL:        "https://www.linkedin.com/in/john-smith",
			WorkEmail:          "john.smith@acme.com",
			VerifiedEmail:      "john.smith@acme.com",
			ConfidenceScore:    65,
			ConfidenceTier:     "Accept-All (Risky)",
			VerificationReason: "Catch-All Server (Standard Corporate Fallback)",
			Status:             "Software Engineer at Acme",
		},
	}

	writtenPath, err := SaveConnectionsToCSV(csvPath, contacts)
	if err != nil {
		t.Fatalf("SaveConnectionsToCSV failed: %v", err)
	}

	if _, err := os.Stat(writtenPath); os.IsNotExist(err) {
		t.Fatalf("CSV file was not created at %s", writtenPath)
	}

	readContacts, err := ReadContactsFromCSV(writtenPath)
	if err != nil {
		t.Fatalf("ReadContactsFromCSV failed: %v", err)
	}

	if len(readContacts) != len(contacts) {
		t.Fatalf("read count = %d, want %d", len(readContacts), len(contacts))
	}

	if readContacts[0].FirstName != "Jane" || readContacts[0].ConfidenceScore != 100 || readContacts[0].ConfidenceTier != "Safe to Send" {
		t.Errorf("read contact mismatch: %+v", readContacts[0])
	}
	if readContacts[1].ConfidenceScore != 65 || readContacts[1].ConfidenceTier != "Accept-All (Risky)" {
		t.Errorf("read contact mismatch: %+v", readContacts[1])
	}
}
