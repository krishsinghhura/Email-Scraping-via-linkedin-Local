package scoring

import (
	"testing"
)

func TestEvaluateScore(t *testing.T) {
	// Case 1: Both verified
	res1 := EvaluateScore(true, true, false, false, false, true, false)
	if res1.Score != 100 || res1.Tier != TierSafeToSend {
		t.Errorf("expected 100 SafeToSend, got %+v", res1)
	}

	// Case 2: Corporate only verified
	res2 := EvaluateScore(false, true, false, false, false, true, false)
	if res2.Score != 98 || res2.Tier != TierSafeToSend {
		t.Errorf("expected 98 SafeToSend, got %+v", res2)
	}

	// Case 3: Personal only verified
	res3 := EvaluateScore(true, false, false, false, false, true, false)
	if res3.Score != 92 || res3.Tier != TierSafeToSend {
		t.Errorf("expected 92 SafeToSend, got %+v", res3)
	}

	// Case 4: Catch-All with verified pattern
	res4 := EvaluateScore(false, true, true, false, true, true, false)
	if res4.Score != 75 || res4.Tier != TierAcceptAll {
		t.Errorf("expected 75 AcceptAll, got %+v", res4)
	}

	// Case 5: Catch-All with default pattern
	res5 := EvaluateScore(false, true, true, false, false, true, false)
	if res5.Score != 65 || res5.Tier != TierAcceptAll {
		t.Errorf("expected 65 AcceptAll, got %+v", res5)
	}

	// Case 6: Catch-All with VRFY confirmed
	res6 := EvaluateScore(false, true, true, true, false, true, false)
	if res6.Score != 85 || res6.Tier != TierHigh {
		t.Errorf("expected 85 High, got %+v", res6)
	}

	// Case 7: Greylisted / Rate-limited
	res7 := EvaluateScore(false, false, false, false, false, true, true)
	if res7.Score != 35 || res7.Tier != TierUncertain {
		t.Errorf("expected 35 Uncertain, got %+v", res7)
	}

	// Case 8: No MX
	res8 := EvaluateScore(false, false, false, false, false, false, false)
	if res8.Score != 0 || res8.Tier != TierUndeliverable {
		t.Errorf("expected 0 Undeliverable, got %+v", res8)
	}
}
