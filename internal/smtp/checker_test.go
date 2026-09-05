package smtp

import (
	"errors"
	"net/textproto"
	"strings"
	"testing"
)

func TestResolveMXRecords(t *testing.T) {
	hosts, err := ResolveMXRecords("google.com")
	if err != nil {
		t.Fatalf("expected google.com MX records, got error: %v", err)
	}
	if len(hosts) == 0 {
		t.Fatalf("expected at least 1 MX host for google.com")
	}

	primary, err := ResolvePrimaryMX("google.com")
	if err != nil {
		t.Fatalf("expected primary MX for google.com, got error: %v", err)
	}
	if primary != hosts[0] {
		t.Fatalf("expected primary %s to match hosts[0] %s", primary, hosts[0])
	}
}

func TestClassifySMTPError(t *testing.T) {
	res250 := classifySMTPError(nil)
	if !res250.Valid || res250.StatusCode != 250 {
		t.Errorf("expected valid 250 result, got %+v", res250)
	}

	err451 := &textproto.Error{Code: 451, Msg: "Greylisting active"}
	res451 := classifySMTPError(err451)
	if res451.Valid || !res451.IsTemporaryErr || res451.StatusCode != 451 {
		t.Errorf("expected temporary error for 451, got %+v", res451)
	}

	err550 := &textproto.Error{Code: 550, Msg: "User not found"}
	res550 := classifySMTPError(err550)
	if res550.Valid || res550.IsTemporaryErr || res550.StatusCode != 550 {
		t.Errorf("expected permanent error for 550, got %+v", res550)
	}

	netErr := errors.New("i/o timeout")
	resNet := classifySMTPError(netErr)
	if resNet.Valid || !resNet.IsTemporaryErr {
		t.Errorf("expected temporary error for network error, got %+v", resNet)
	}
}

func TestRandomMailboxGeneration(t *testing.T) {
	mb1 := GenerateRandomMailbox("example.com")
	mb2 := GenerateSecondaryRandomMailbox("example.com")

	if mb1 == mb2 {
		t.Errorf("expected different random mailboxes, got %s and %s", mb1, mb2)
	}
	if !strings.HasSuffix(mb1, "@example.com") || !strings.HasSuffix(mb2, "@example.com") {
		t.Errorf("expected @example.com suffix, got %s, %s", mb1, mb2)
	}
}

