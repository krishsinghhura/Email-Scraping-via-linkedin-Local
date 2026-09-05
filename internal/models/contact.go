package models

type Contact struct {
	RowIndex           int
	FirstName          string
	LastName           string
	Domain             string
	LinkedInURL        string
	PersonalEmail      string
	WorkEmail          string
	VerifiedEmail      string
	ConfidenceScore    int    // 0 - 100
	ConfidenceTier     string // "Safe to Send", "High Confidence", "Accept-All (Risky)", "Uncertain", "Undeliverable"
	VerificationReason string // Explanatory technical diagnosis
	Status             string
	CampaignSent       string
}
