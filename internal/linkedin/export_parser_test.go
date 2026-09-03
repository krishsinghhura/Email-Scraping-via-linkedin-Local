package linkedin

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLinkedInExportCSV(t *testing.T) {
	sampleData := []byte(`Notes:
When exporting your connection data, that you have chosen to include...

First Name,Last Name,URL,Email Address,Company,Position,Connected On
Jane,Doe,https://www.linkedin.com/in/janedoe,,Google,Product Lead,01 Jan 2024
Bob,Smith,https://www.linkedin.com/in/bobsmith,bob@stripe.com,Stripe,Senior Engineer,15 Dec 2023
`)

	contacts, err := ParseLinkedInExportCSV(sampleData)
	if err != nil {
		t.Fatalf("unexpected error parsing LinkedIn export: %v", err)
	}

	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(contacts))
	}

	if contacts[0].FirstName != "Jane" || contacts[0].LastName != "Doe" {
		t.Errorf("expected Jane Doe, got %s %s", contacts[0].FirstName, contacts[0].LastName)
	}
	if contacts[0].LinkedInURL != "https://www.linkedin.com/in/janedoe" {
		t.Errorf("unexpected URL: %s", contacts[0].LinkedInURL)
	}
	if contacts[1].VerifiedEmail != "bob@stripe.com" {
		t.Errorf("expected bob@stripe.com, got %s", contacts[1].VerifiedEmail)
	}
}

func TestParseLinkedInExportZip(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "LinkedInDataExport.zip")

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create("Connections.csv")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("First Name,Last Name,URL,Email Address,Company,Position,Connected On\nAlice,Wonderland,https://www.linkedin.com/in/alice,,Apple,Designer,05 May 2024\n")
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	w.Close()

	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	contacts, name, err := ParseLinkedInExport(zipPath)
	if err != nil {
		t.Fatalf("failed to parse zip export: %v", err)
	}

	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(contacts))
	}
	if contacts[0].FirstName != "Alice" {
		t.Errorf("expected Alice, got %s", contacts[0].FirstName)
	}
	if name != "LinkedInDataExport" {
		t.Errorf("unexpected name: %s", name)
	}
}
