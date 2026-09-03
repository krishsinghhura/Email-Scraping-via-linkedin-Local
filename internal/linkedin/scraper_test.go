package linkedin

import (
	"testing"
)

func TestParseTitleString(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		wantName      string
		wantCompany   string
		wantFirstName string
		wantLastName  string
	}{
		{
			name:          "Standard LinkedIn Title",
			title:         "Jane Doe - Senior Product Manager - Google | LinkedIn",
			wantName:      "Jane Doe",
			wantCompany:   "Google",
			wantFirstName: "Jane",
			wantLastName:  "Doe",
		},
		{
			name:          "Title with hyphen company",
			title:         "John Smith - Software Engineer - ExampleTech | LinkedIn",
			wantName:      "John Smith",
			wantCompany:   "ExampleTech",
			wantFirstName: "John",
			wantLastName:  "Smith",
		},
		{
			name:          "Title with Inc suffix",
			title:         "Alex Taylor - VP Engineering - Acme Corp Inc. | LinkedIn",
			wantName:      "Alex Taylor",
			wantCompany:   "Acme Corp",
			wantFirstName: "Alex",
			wantLastName:  "Taylor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &ProfileMetadata{}
			parseTitleString(tt.title, meta)

			if meta.FullName != tt.wantName {
				t.Errorf("FullName = %q, want %q", meta.FullName, tt.wantName)
			}
			if meta.Company != tt.wantCompany {
				t.Errorf("Company = %q, want %q", meta.Company, tt.wantCompany)
			}
			if meta.FirstName != tt.wantFirstName {
				t.Errorf("FirstName = %q, want %q", meta.FirstName, tt.wantFirstName)
			}
			if meta.LastName != tt.wantLastName {
				t.Errorf("LastName = %q, want %q", meta.LastName, tt.wantLastName)
			}
		})
	}
}

func TestExtractCompanyFromDescription(t *testing.T) {
	desc := "View Jane Doe's profile on LinkedIn. Jane is currently Senior Product Manager at Google in San Francisco."
	comp := extractCompanyFromDescription(desc)
	if comp != "Google" {
		t.Errorf("extracted company = %q, want %q", comp, "Google")
	}
}
