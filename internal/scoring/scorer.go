package scoring

type ScoreResult struct {
	Score  int
	Tier   string
	Reason string
}

const (
	TierSafeToSend    = "Safe to Send"
	TierHigh          = "High Confidence"
	TierAcceptAll     = "Accept-All (Risky)"
	TierUncertain     = "Uncertain (Deferred)"
	TierUndeliverable = "Undeliverable"
)

// EvaluateScore determines the deliverability score (0-100), tier, and technical reason
func EvaluateScore(
	hasPersonalEmail bool,
	hasWorkEmail bool,
	workIsCatchAll bool,
	vrfyConfirmed bool,
	patternMatched bool,
	hasValidMX bool,
	tempErrorOccurred bool,
) ScoreResult {
	// Case 1: Direct corporate SMTP verified (non-catch-all)
	if hasWorkEmail && !workIsCatchAll {
		if hasPersonalEmail {
			return ScoreResult{
				Score:  100,
				Tier:   TierSafeToSend,
				Reason: "SMTP 250 OK (Personal & Corporate Mailboxes Confirmed)",
			}
		}
		return ScoreResult{
			Score:  98,
			Tier:   TierSafeToSend,
			Reason: "SMTP 250 OK (Corporate Mailbox Confirmed)",
		}
	}

	// Case 2: Direct personal email verified (Gmail / Outlook)
	if hasPersonalEmail {
		if hasWorkEmail && workIsCatchAll {
			if vrfyConfirmed {
				return ScoreResult{
					Score:  95,
					Tier:   TierSafeToSend,
					Reason: "SMTP 250 OK (Personal Confirmed + Corporate VRFY Confirmed)",
				}
			}
			return ScoreResult{
				Score:  90,
				Tier:   TierHigh,
				Reason: "SMTP 250 OK (Personal Confirmed) + Corporate Accept-All",
			}
		}
		return ScoreResult{
			Score:  92,
			Tier:   TierSafeToSend,
			Reason: "SMTP 250 OK (Personal Mailbox Confirmed)",
		}
	}

	// Case 3: Work email is on a Catch-All server
	if hasWorkEmail && workIsCatchAll {
		if vrfyConfirmed {
			return ScoreResult{
				Score:  85,
				Tier:   TierHigh,
				Reason: "Catch-All Server (SMTP VRFY Mailbox Confirmed)",
			}
		}
		if patternMatched {
			return ScoreResult{
				Score:  75,
				Tier:   TierAcceptAll,
				Reason: "Catch-All Server (Verified Company Pattern Applied)",
			}
		}
		return ScoreResult{
			Score:  65,
			Tier:   TierAcceptAll,
			Reason: "Catch-All Server (Standard Corporate Fallback)",
		}
	}

	// Case 4: No mailbox confirmed, but temporary error (greylisting / rate-limit)
	if tempErrorOccurred && hasValidMX {
		return ScoreResult{
			Score:  35,
			Tier:   TierUncertain,
			Reason: "Server Greylisted / Connection Rate-Limited (4xx)",
		}
	}

	// Case 5: No MX records or mailbox rejected
	if !hasValidMX {
		return ScoreResult{
			Score:  0,
			Tier:   TierUndeliverable,
			Reason: "MX Lookup Failed (Domain Does Not Accept Email)",
		}
	}

	return ScoreResult{
		Score:  0,
		Tier:   TierUndeliverable,
		Reason: "Mailbox Rejected (550 User Unknown)",
	}
}
