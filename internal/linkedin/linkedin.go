package linkedin

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"email-verifier-cli/internal/permutator"
	"email-verifier-cli/internal/smtp"
)

type ProfileInfo struct {
	URL       string
	Slug      string
	FirstName string
	LastName  string
	Company   string
	Domain    string
}

func ParseProfileURL(rawURL string) (*ProfileInfo, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("empty LinkedIn URL")
	}

	formattedURL := trimmed
	if !strings.HasPrefix(formattedURL, "http://") && !strings.HasPrefix(formattedURL, "https://") {
		formattedURL = "https://" + formattedURL
	}

	parsed, err := url.Parse(formattedURL)
	if err != nil {
		return nil, fmt.Errorf("invalid LinkedIn URL: %w", err)
	}

	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")
	var slug string
	for i, part := range parts {
		if (part == "in" || part == "pub") && i+1 < len(parts) {
			slug = parts[i+1]
			break
		}
	}
	if slug == "" && len(parts) > 0 {
		slug = parts[len(parts)-1]
	}

	slug = strings.Split(slug, "?")[0]
	slug = strings.Split(slug, "#")[0]

	if slug == "" {
		return nil, fmt.Errorf("could not extract profile slug from LinkedIn URL")
	}

	fn, ln := parseNameFromSlug(slug)
	if fn == "" {
		return nil, fmt.Errorf("could not parse valid name from profile slug '%s'", slug)
	}

	return &ProfileInfo{
		URL:       rawURL,
		Slug:      slug,
		FirstName: fn,
		LastName:  ln,
	}, nil
}

func parseNameFromSlug(slug string) (string, string) {
	slug = strings.ToLower(slug)

	reID := regexp.MustCompile(`-[0-9a-f]{5,}$|-[0-9]{3,}$|[0-9]+$`)
	slug = reID.ReplaceAllString(slug, "")

	parts := regexp.MustCompile(`[-_]+`).Split(slug, -1)
	var filtered []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		return "", ""
	}
	if len(filtered) == 1 {
		return capitalize(filtered[0]), ""
	}

	firstName := capitalize(filtered[0])
	var lastNameParts []string
	for _, p := range filtered[1:] {
		lastNameParts = append(lastNameParts, capitalize(p))
	}

	return firstName, strings.Join(lastNameParts, " ")
}

func capitalize(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= 2 {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])
}

func FindEmailFromLinkedIn(linkedinURL, domain, senderDomain string, timeout time.Duration) (string, []string, error) {
	info, err := ParseProfileURL(linkedinURL)
	if err != nil {
		return "", nil, err
	}

	if domain == "" {
		return "", nil, fmt.Errorf("domain is required to verify emails for %s %s", info.FirstName, info.LastName)
	}

	candidates := GetCandidatePermutations(info.FirstName, info.LastName, domain)

	mxHost, err := smtp.ResolvePrimaryMX(domain)
	if err != nil {
		return "", candidates, fmt.Errorf("MX lookup failed for domain '%s': %w", domain, err)
	}

	if smtp.IsCatchAll(mxHost, domain, senderDomain, timeout) {
		return "", candidates, fmt.Errorf("domain '%s' is Catch-All (all mailboxes accepted)", domain)
	}

	for _, candidate := range candidates {
		if smtp.ProbeMailbox(mxHost, candidate, senderDomain, timeout) {
			return candidate, candidates, nil
		}
	}

	return "", candidates, fmt.Errorf("no valid email permutation found for %s %s @%s", info.FirstName, info.LastName, domain)
}

func GetCandidatePermutations(firstName, lastName, domain string) []string {
	candidates := permutator.GeneratePermutations(firstName, lastName, domain)

	if strings.Contains(lastName, " ") {
		parts := strings.Fields(lastName)
		if len(parts) >= 2 {
			more1 := permutator.GeneratePermutations(firstName, parts[0], domain)
			more2 := permutator.GeneratePermutations(firstName, parts[len(parts)-1], domain)
			more3 := permutator.GeneratePermutations(firstName, strings.Join(parts, ""), domain)

			candidates = append(candidates, more1...)
			candidates = append(candidates, more2...)
			candidates = append(candidates, more3...)
		}
	}

	seen := make(map[string]bool)
	var deduped []string
	for _, c := range candidates {
		if !seen[c] {
			seen[c] = true
			deduped = append(deduped, c)
		}
	}
	return deduped
}
