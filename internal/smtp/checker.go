package smtp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	netsmtp "net/smtp"
	"sort"
	"strings"
	"time"
)

func ResolvePrimaryMX(domain string) (string, error) {
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return "", fmt.Errorf("no MX records found: %w", err)
	}

	sort.Slice(mxRecords, func(i, j int) bool {
		return mxRecords[i].Pref < mxRecords[j].Pref
	})

	return strings.TrimSuffix(mxRecords[0].Host, "."), nil
}

func ProbeMailbox(mxHost, targetEmail, senderDomain string, timeout time.Duration) bool {
	addr := net.JoinHostPort(mxHost, "25")
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	client, err := netsmtp.NewClient(conn, mxHost)
	if err != nil {
		return false
	}
	defer client.Close()

	if err := client.Hello(senderDomain); err != nil {
		return false
	}

	if err := client.Mail(fmt.Sprintf("verify@%s", senderDomain)); err != nil {
		return false
	}

	if err := client.Rcpt(targetEmail); err != nil {
		return false
	}

	_ = client.Reset()
	_ = client.Quit()
	return true
}

func GenerateRandomMailbox(domain string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("random_check_xyz123@%s", domain)
	}
	return fmt.Sprintf("catchall_chk_%s@%s", hex.EncodeToString(bytes), domain)
}

func IsCatchAll(mxHost, domain, senderDomain string, timeout time.Duration) bool {
	probeEmail := GenerateRandomMailbox(domain)
	return ProbeMailbox(mxHost, probeEmail, senderDomain, timeout)
}
