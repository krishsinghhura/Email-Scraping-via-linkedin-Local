package config

import (
	"testing"
)

func TestCleanCookieValue(t *testing.T) {
	val := CleanCookieValue(` "ajax:12345" `)
	if val != "ajax:12345" {
		t.Errorf("CleanCookieValue = %q, want 'ajax:12345'", val)
	}
}
