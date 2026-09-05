package smtp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	netsmtp "net/smtp"
	"net/textproto"
	"sort"
	"strings"
	"time"
)

type ProbeResult struct {
	Valid          bool
	IsTemporaryErr bool
	StatusCode     int
	ErrorMsg       string
}

func ResolveMXRecords(domain string) ([]string, error) {
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return nil, fmt.Errorf("no MX records found: %w", err)
	}

	sort.Slice(mxRecords, func(i, j int) bool {
		return mxRecords[i].Pref < mxRecords[j].Pref
	})

	var hosts []string
	for _, rec := range mxRecords {
		h := strings.TrimSuffix(rec.Host, ".")
		if h != "" {
			hosts = append(hosts, h)
		}
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no valid MX host addresses")
	}

	return hosts, nil
}

func ResolvePrimaryMX(domain string) (string, error) {
	hosts, err := ResolveMXRecords(domain)
	if err != nil {
		return "", err
	}
	return hosts[0], nil
}

func ProbeMailboxSingle(mxHost, targetEmail, senderDomain string, timeout time.Duration) ProbeResult {
	addr := net.JoinHostPort(mxHost, "25")
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return ProbeResult{Valid: false, IsTemporaryErr: true, ErrorMsg: err.Error()}
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	client, err := netsmtp.NewClient(conn, mxHost)
	if err != nil {
		return ProbeResult{Valid: false, IsTemporaryErr: true, ErrorMsg: err.Error()}
	}
	defer client.Close()

	if err := client.Hello(senderDomain); err != nil {
		return classifySMTPError(err)
	}

	fromAddr := fmt.Sprintf("verify@%s", senderDomain)
	if err := client.Mail(fromAddr); err != nil {
		return classifySMTPError(err)
	}

	if err := client.Rcpt(targetEmail); err != nil {
		return classifySMTPError(err)
	}

	_ = client.Reset()
	_ = client.Quit()

	return ProbeResult{Valid: true, StatusCode: 250}
}

func classifySMTPError(err error) ProbeResult {
	if err == nil {
		return ProbeResult{Valid: true, StatusCode: 250}
	}

	var code int
	if tpErr, ok := err.(*textproto.Error); ok {
		code = tpErr.Code
	}

	isTemp := false
	if code >= 400 && code < 500 {
		isTemp = true
	} else if code == 0 {
		isTemp = true
	}

	return ProbeResult{
		Valid:          false,
		IsTemporaryErr: isTemp,
		StatusCode:     code,
		ErrorMsg:       err.Error(),
	}
}

func ProbeMailboxMultiMX(mxHosts []string, targetEmail, senderDomain string, timeout time.Duration) bool {
	if len(mxHosts) == 0 {
		return false
	}

	limit := len(mxHosts)
	if limit > 3 {
		limit = 3
	}

	for i := 0; i < limit; i++ {
		host := mxHosts[i]
		res := ProbeMailboxSingle(host, targetEmail, senderDomain, timeout)
		if res.Valid {
			return true
		}

		if !res.IsTemporaryErr && res.StatusCode >= 500 {
			return false
		}

		if res.IsTemporaryErr && (res.StatusCode == 451 || res.StatusCode == 421 || res.StatusCode == 450) {
			time.Sleep(300 * time.Millisecond)
			retryRes := ProbeMailboxSingle(host, targetEmail, senderDomain, timeout)
			if retryRes.Valid {
				return true
			}
			if !retryRes.IsTemporaryErr && retryRes.StatusCode >= 500 {
				return false
			}
		}
	}

	return false
}

func ProbeMailbox(mxHost, targetEmail, senderDomain string, timeout time.Duration) bool {
	return ProbeMailboxMultiMX([]string{mxHost}, targetEmail, senderDomain, timeout)
}

func GenerateRandomMailbox(domain string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("catchall_chk_xyz123@%s", domain)
	}
	return fmt.Sprintf("catchall_chk_%s@%s", hex.EncodeToString(b), domain)
}

func GenerateSecondaryRandomMailbox(domain string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("probe_fake_xyz123@%s", domain)
	}
	return fmt.Sprintf("probe_fake_%s@%s", hex.EncodeToString(b), domain)
}

func IsCatchAllMultiMX(mxHosts []string, domain, senderDomain string, timeout time.Duration) bool {
	probe1 := GenerateRandomMailbox(domain)
	if !ProbeMailboxMultiMX(mxHosts, probe1, senderDomain, timeout) {
		return false
	}

	// Secondary check with distinct prefix to confirm catch-all policy
	probe2 := GenerateSecondaryRandomMailbox(domain)
	return ProbeMailboxMultiMX(mxHosts, probe2, senderDomain, timeout)
}

func IsCatchAll(mxHost, domain, senderDomain string, timeout time.Duration) bool {
	return IsCatchAllMultiMX([]string{mxHost}, domain, senderDomain, timeout)
}

func ProbeVRFY(mxHost, targetEmail, senderDomain string, timeout time.Duration) (valid bool, supported bool) {
	addr := net.JoinHostPort(mxHost, "25")
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false, false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	client, err := netsmtp.NewClient(conn, mxHost)
	if err != nil {
		return false, false
	}
	defer client.Close()

	if err := client.Hello(senderDomain); err != nil {
		return false, false
	}

	errVRFY := client.Verify(targetEmail)
	if errVRFY == nil {
		return true, true
	}

	if tpErr, ok := errVRFY.(*textproto.Error); ok {
		if tpErr.Code == 250 || tpErr.Code == 251 {
			return true, true
		}
		if tpErr.Code == 550 || tpErr.Code == 551 || tpErr.Code == 553 {
			return false, true
		}
	}

	return false, false
}

