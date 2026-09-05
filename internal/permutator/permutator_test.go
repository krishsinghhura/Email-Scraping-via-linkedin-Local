package permutator

import (
	"testing"
)

func TestGeneratePermutations(t *testing.T) {
	perms := GeneratePermutations("Jane", "Doe", "company.com")

	if len(perms) == 0 {
		t.Fatalf("expected non-empty permutations")
	}

	expected := []string{
		"jane.doe@company.com",
		"jane@company.com",
		"janedoe@company.com",
		"jdoe@company.com",
		"jane_doe@company.com",
		"doe@company.com",
		"doe.jane@company.com",
		"j.doe@company.com",
		"doej@company.com",
		"janed@company.com",
		"jane.d@company.com",
		"doe_jane@company.com",
		"doejane@company.com",
		"jane-doe@company.com",
		"j-doe@company.com",
	}

	permMap := make(map[string]bool)
	for _, p := range perms {
		permMap[p] = true
	}

	for _, exp := range expected {
		if !permMap[exp] {
			t.Errorf("missing expected permutation pattern %s", exp)
		}
	}
}

func TestDetectAndFormatPattern(t *testing.T) {
	tests := []struct {
		email     string
		firstName string
		lastName  string
		expected  string
	}{
		{"kaushik.das@deloitte.com", "Kaushik", "Das", "{first}.{last}"},
		{"kdas@deloitte.com", "Kaushik", "Das", "{finit}{last}"},
		{"kaushik@deloitte.com", "Kaushik", "Das", "{first}"},
		{"das.kaushik@deloitte.com", "Kaushik", "Das", "{last}.{first}"},
		{"kaushikd@deloitte.com", "Kaushik", "Das", "{first}{linit}"},
	}

	for _, tc := range tests {
		pat := DetectPattern(tc.email, tc.firstName, tc.lastName)
		if pat != tc.expected {
			t.Errorf("DetectPattern(%q, %q, %q) = %q, want %q", tc.email, tc.firstName, tc.lastName, pat, tc.expected)
		}

		formatted := FormatPattern(pat, tc.firstName, tc.lastName, "deloitte.com")
		if formatted != tc.email {
			t.Errorf("FormatPattern(%q) = %q, want %q", pat, formatted, tc.email)
		}
	}
}
