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
			FirstName:   "Jane",
			LastName:    "Doe",
			Domain:      "example.com",
			LinkedInURL: "https://www.linkedin.com/in/jane-doe",
			Status:      "Senior Product Manager at ExampleCorp",
		},
		{
			FirstName:   "John",
			LastName:    "Smith",
			Domain:      "acme.com",
			LinkedInURL: "https://www.linkedin.com/in/john-smith",
			Status:      "Software Engineer at Acme",
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

	if readContacts[0].FirstName != "Jane" || readContacts[0].Domain != "example.com" {
		t.Errorf("read contact mismatch: %+v", readContacts[0])
	}
}
