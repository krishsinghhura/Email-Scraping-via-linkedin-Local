package models

type Contact struct {
	RowIndex           int
	FirstName          string
	LastName           string
	Domain             string
	LinkedInURL        string
	RegisteredEmail    string // Officially registered email on LinkedIn profile contact info / export
	PersonalEmail      string
	WorkEmail          string
	VerifiedEmail      string
	ConfidenceScore    int    // 0 - 100
	ConfidenceTier     string // "Safe to Send", "High Confidence", "Accept-All (Risky)", "Personal Guess (Unconfirmed Identity)", "Undeliverable"
	VerificationReason string // Explanatory technical diagnosis
	Status             string
	CampaignSent       string
	IsPersonalGuessed  bool   // True if PersonalEmail was guessed from permutations rather than from contact info
}
