package linkedin

import (
	"testing"
)

func TestSanitizeCompanyName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Apexon (a Goldman Sachs and Everstone Company)", "Apexon"},
		{"Veer Surendra Sai University of Technology (formerly UCE Burla)", "Veer Surendra Sai University of Technology"},
		{"GWC DATA.AI [Snowflake, Claude, Domo...]", "GWC DATA.AI"},
		{"Leena AI Pvt Ltd", "Leena AI"},
		{"Google Inc.", "Google"},
		{"Tata Elxsi Limited Pune", "Tata Elxsi"},
	}

	for _, tc := range tests {
		got := SanitizeCompanyName(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeCompanyName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestExtractRootDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://www.apexon.com/about/", "apexon.com"},
		{"http://sub.domain.co.uk/path?param=1", "sub.domain.co.uk"},
		{"google.com", "google.com"},
		{"https://www.leena.ai", "leena.ai"},
	}

	for _, tc := range tests {
		got := ExtractRootDomain(tc.input)
		if got != tc.expected {
			t.Errorf("ExtractRootDomain(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestResolveCompanyDomain(t *testing.T) {
	domain, err := ResolveCompanyDomain("Google")
	if err != nil {
		t.Fatalf("ResolveCompanyDomain('Google') failed: %v", err)
	}
	if domain != "google.com" {
		t.Errorf("domain = %q, want 'google.com'", domain)
	}
}
