package linkedin

import (
	"testing"
)

func TestParseProfileURL(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		wantFirstName string
		wantLastName  string
		wantSlug      string
		wantErr       bool
	}{
		{
			name:          "Standard profile URL",
			rawURL:        "https://www.linkedin.com/in/jane-doe",
			wantFirstName: "Jane",
			wantLastName:  "Doe",
			wantSlug:      "jane-doe",
			wantErr:       false,
		},
		{
			name:          "Profile with trailing slash and query params",
			rawURL:        "https://www.linkedin.com/in/jane-doe/?originalSubdomain=us",
			wantFirstName: "Jane",
			wantLastName:  "Doe",
			wantSlug:      "jane-doe",
			wantErr:       false,
		},
		{
			name:          "Profile with hex ID suffix",
			rawURL:        "https://linkedin.com/in/john-smith-12345678",
			wantFirstName: "John",
			wantLastName:  "Smith",
			wantSlug:      "john-smith-12345678",
			wantErr:       false,
		},
		{
			name:          "Multi-word slug",
			rawURL:        "https://www.linkedin.com/in/john-edward-smith",
			wantFirstName: "John",
			wantLastName:  "Edward Smith",
			wantSlug:      "john-edward-smith",
			wantErr:       false,
		},
		{
			name:    "Empty input",
			rawURL:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseProfileURL(tt.rawURL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseProfileURL(%q) error = %v, wantErr = %v", tt.rawURL, err, tt.wantErr)
			}
			if !tt.wantErr {
				if info.FirstName != tt.wantFirstName {
					t.Errorf("FirstName = %q, want %q", info.FirstName, tt.wantFirstName)
				}
				if info.LastName != tt.wantLastName {
					t.Errorf("LastName = %q, want %q", info.LastName, tt.wantLastName)
				}
				if info.Slug != tt.wantSlug {
					t.Errorf("Slug = %q, want %q", info.Slug, tt.wantSlug)
				}
			}
		})
	}
}

func TestGetCandidatePermutations(t *testing.T) {
	perms := GetCandidatePermutations("Jane", "Doe", "example.com")
	if len(perms) == 0 {
		t.Fatalf("expected non-empty permutations for Jane Doe")
	}

	expectedPattern := "jane.doe@example.com"
	found := false
	for _, p := range perms {
		if p == expectedPattern {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected permutation %q in candidate list %v", expectedPattern, perms)
	}
}
