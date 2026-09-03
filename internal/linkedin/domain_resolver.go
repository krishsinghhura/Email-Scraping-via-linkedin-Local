package linkedin

import (
	"fmt"
	"regexp"
	"strings"

	"email-verifier-cli/internal/smtp"
)

func checkDomainMX(domain string) bool {
	hosts, err := smtp.ResolveMXRecords(domain)
	return err == nil && len(hosts) > 0
}

func ResolveCompanyDomain(companyName string) (string, error) {
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return "", fmt.Errorf("empty company name")
	}

	lower := strings.ToLower(companyName)

	if strings.Contains(lower, ".") && !strings.Contains(lower, " ") {
		clean := strings.Trim(lower, " ./@")
		if checkDomainMX(clean) {
			return clean, nil
		}
	}

	cleanupWords := []string{
		" pvt ltd", " private limited", " ltd", " limited",
		" inc", " incorporated", " llc", " corp", " corporation",
		" technologies", " technology", " solutions", " global",
		" pune", " bhubaneswar", " bangalore", " bengaluru", " mumbai", " delhi", " hyderabad", " chennai", " india",
	}
	cleaned := lower
	for _, w := range cleanupWords {
		if strings.HasSuffix(cleaned, w) {
			cleaned = strings.TrimSuffix(cleaned, w)
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	var candidates []string

	if strings.HasSuffix(cleaned, " ai") {
		base := strings.TrimSpace(strings.TrimSuffix(cleaned, " ai"))
		reNonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
		slug := reNonAlpha.ReplaceAllString(base, "")
		if slug != "" {
			candidates = append(candidates, slug+".ai")
		}
	} else if strings.HasSuffix(cleaned, " io") {
		base := strings.TrimSpace(strings.TrimSuffix(cleaned, " io"))
		reNonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
		slug := reNonAlpha.ReplaceAllString(base, "")
		if slug != "" {
			candidates = append(candidates, slug+".io")
		}
	}

	reNonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
	fullSlug := reNonAlpha.ReplaceAllString(cleaned, "")
	if fullSlug != "" && len(fullSlug) <= 25 {
		candidates = append(candidates,
			fullSlug+".com",
			fullSlug+".ai",
			fullSlug+".io",
			fullSlug+".co",
			fullSlug+".in",
			fullSlug+".org",
		)
	}

	words := strings.Fields(cleaned)
	if len(words) >= 2 {
		twoWords := reNonAlpha.ReplaceAllString(words[0]+words[1], "")
		if len(twoWords) >= 5 && len(twoWords) <= 25 {
			candidates = append(candidates,
				twoWords+".com",
				twoWords+".in",
			)
		}
	}
	if len(words) > 1 {
		firstWord := reNonAlpha.ReplaceAllString(words[0], "")
		if len(firstWord) >= 3 && len(firstWord) <= 20 {
			candidates = append(candidates,
				firstWord+".com",
				firstWord+".ai",
				firstWord+".io",
			)
		}
	}

	seen := make(map[string]bool)
	for _, cand := range candidates {
		cand = strings.TrimSpace(cand)
		if cand == "" || seen[cand] {
			continue
		}
		seen[cand] = true

		if checkDomainMX(cand) {
			return cand, nil
		}
	}

	return "", fmt.Errorf("could not resolve domain for %s", companyName)
}
