package linkedin

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"email-verifier-cli/internal/smtp"
)

func ResolveCompanyDomain(companyName string) (string, error) {
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return "", fmt.Errorf("empty company name")
	}

	slug := strings.ToLower(companyName)
	reNonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
	cleanSlug := reNonAlpha.ReplaceAllString(slug, "")

	if cleanSlug == "" {
		return "", fmt.Errorf("invalid company slug for '%s'", companyName)
	}

	tlds := []string{".com", ".in", ".io", ".co", ".ai", ".org", ".net"}
	for _, tld := range tlds {
		candidateDomain := cleanSlug + tld

		_, errMX := smtp.ResolvePrimaryMX(candidateDomain)
		if errMX == nil {
			return candidateDomain, nil
		}

		_, errHost := net.LookupHost(candidateDomain)
		if errHost == nil {
			return candidateDomain, nil
		}
	}

	return cleanSlug + ".com", nil
}
