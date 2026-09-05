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
	TierPersonalGuess = "Personal Guess (Unconfirmed Identity)"
	TierUncertain     = "Uncertain (Deferred)"
	TierUndeliverable = "Undeliverable"
)

// EvaluateScore determines the deliverability score (0-100), tier, and technical reason
func EvaluateScore(
	hasRegisteredEmail bool,
	hasPersonalEmail bool,
	personalIsGuessed bool,
	hasWorkEmail bool,
	workIsCatchAll bool,
	vrfyConfirmed bool,
	patternMatched bool,
	hasValidMX bool,
	tempErrorOccurred bool,
) ScoreResult {
	// Case 0: Official LinkedIn Registered Email confirmed
	if hasRegisteredEmail {
		if hasWorkEmail && !workIsCatchAll {
			return ScoreResult{
				Score:  100,
				Tier:   TierSafeToSend,
				Reason: "LinkedIn Profile Confirmed + Corporate Mailbox Confirmed",
			}
		}
		return ScoreResult{
			Score:  100,
			Tier:   TierSafeToSend,
			Reason: "SMTP 250 OK (LinkedIn Profile Contact Info Confirmed)",
		}
	}

	// Case 1: Direct corporate SMTP verified (non-catch-all)
	if hasWorkEmail && !workIsCatchAll {
		if hasPersonalEmail && !personalIsGuessed {
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

	// Case 2: Corporate Catch-All with verified mailbox or pattern
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

	// Case 3: Personal email verified
	if hasPersonalEmail {
		if !personalIsGuessed {
			return ScoreResult{
				Score:  92,
				Tier:   TierSafeToSend,
				Reason: "SMTP 250 OK (Personal Mailbox Confirmed)",
			}
		}
		// Purely guessed on public provider (gmail/outlook)
		return ScoreResult{
			Score:  50,
			Tier:   TierPersonalGuess,
			Reason: "Mailbox exists on public provider (@gmail/@outlook), but unconfirmed identity (common name collision risk)",
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
