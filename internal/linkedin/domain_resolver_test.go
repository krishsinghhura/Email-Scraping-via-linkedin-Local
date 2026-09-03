package linkedin

import (
	"testing"
)

func TestResolveCompanyDomain(t *testing.T) {
	domain, err := ResolveCompanyDomain("Google")
	if err != nil {
		t.Fatalf("ResolveCompanyDomain('Google') failed: %v", err)
	}
	if domain != "google.com" {
		t.Errorf("domain = %q, want 'google.com'", domain)
	}
}
