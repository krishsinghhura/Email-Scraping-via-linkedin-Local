package linkedin

import (
	"testing"
)

func TestParseFlexibleConnectionsJSON(t *testing.T) {
	mockPayload := []byte(`{
		"elements": [
			{
				"miniProfile": {
					"firstName": "Alice",
					"lastName": "Smith",
					"occupation": "Engineering Director at Google",
					"publicIdentifier": "alicesmith"
				}
			}
		],
		"included": [
			{
				"$type": "com.linkedin.voyager.dash.identity.profile.Profile",
				"firstName": "Bob",
				"lastName": "Jones",
				"primarySubtitle": {"text": "Product Lead at Stripe"},
				"publicIdentifier": "bobjones"
			},
			{
				"$type": "com.linkedin.voyager.identity.shared.MiniProfile",
				"title": {"text": "Charlie Brown"},
				"occupation": "Developer at Apple",
				"publicIdentifier": "charliebrown"
			}
		]
	}`)

	contacts := parseFlexibleConnectionsJSON(mockPayload)
	if len(contacts) != 3 {
		t.Fatalf("expected 3 contacts parsed, got %d", len(contacts))
	}

	names := map[string]bool{
		"Alice Smith":    false,
		"Bob Jones":      false,
		"Charlie Brown":  false,
	}

	for _, c := range contacts {
		full := c.FirstName + " " + c.LastName
		names[full] = true
	}

	for name, found := range names {
		if !found {
			t.Errorf("missing expected contact %q", name)
		}
	}
}
