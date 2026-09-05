package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.json")

	store1 := NewStore(cachePath)
	store1.SetCompanyDomain("Google Inc.", "google.com")
	store1.SetDomainPattern("google.com", "{first}.{last}")

	dom, ok := store1.GetCompanyDomain("google inc")
	if !ok || dom != "google.com" {
		t.Fatalf("expected google.com, got %s (ok=%v)", dom, ok)
	}

	pat, ok := store1.GetDomainPattern("google.com")
	if !ok || pat != "{first}.{last}" {
		t.Fatalf("expected {first}.{last}, got %s (ok=%v)", pat, ok)
	}

	// Reload in a new store instance to test disk persistence
	store2 := NewStore(cachePath)
	dom2, ok := store2.GetCompanyDomain("  google   inc. ")
	if !ok || dom2 != "google.com" {
		t.Fatalf("persisted domain expected google.com, got %s (ok=%v)", dom2, ok)
	}

	pat2, ok := store2.GetDomainPattern("GOOGLE.COM")
	if !ok || pat2 != "{first}.{last}" {
		t.Fatalf("persisted pattern expected {first}.{last}, got %s (ok=%v)", pat2, ok)
	}

	_ = os.Remove(cachePath)
}
