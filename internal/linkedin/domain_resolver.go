package linkedin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"email-verifier-cli/internal/cache"
	"email-verifier-cli/internal/smtp"
)

var httpClient = &http.Client{
	Timeout: 4 * time.Second,
}

func checkDomainMX(domain string) bool {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" || strings.Contains(domain, " ") || !strings.Contains(domain, ".") {
		return false
	}
	hosts, err := smtp.ResolveMXRecords(domain)
	return err == nil && len(hosts) > 0
}

func ExtractRootDomain(rawURL string) string {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	return host
}

func SanitizeCompanyName(companyName string) string {
	name := strings.TrimSpace(companyName)
	if name == "" {
		return ""
	}

	// Strip bracketed text: [Snowflake, Claude, Domo...] or (a Goldman Sachs...)
	reBrackets := regexp.MustCompile(`\[.*?\]|\(.*?\)|\{.*?\}`)
	name = reBrackets.ReplaceAllString(name, "")

	// Strip suffixes after dashes or delimiters: " - Snowflake...", " | ..."
	delims := []string{" - ", " – ", " — ", " | ", " • ", " · ", " / ", " formerly ", " ex-", " ex "}
	for _, d := range delims {
		if idx := strings.Index(strings.ToLower(name), d); idx != -1 {
			name = name[:idx]
		}
	}

	cleanupWords := []string{
		"pvt ltd", "private limited", "ltd.", "ltd", "limited",
		"inc.", "inc", "incorporated", "llc.", "llc", "corp.", "corp", "corporation",
		"technologies", "technology", "solutions", "global", "group",
		"pune", "bhubaneswar", "bangalore", "bengaluru", "mumbai", "delhi", "hyderabad", "chennai", "india",
	}

	changed := true
	for changed {
		changed = false
		name = strings.Trim(strings.TrimSpace(name), " .,-@|!#")
		lower := strings.ToLower(name)
		for _, w := range cleanupWords {
			if strings.HasSuffix(lower, " "+w) {
				if (w == "technology" || w == "technologies") && (strings.HasSuffix(lower, "university of "+w) || strings.HasSuffix(lower, "institute of "+w)) {
					continue
				}
				name = strings.TrimSpace(name[:len(name)-len(w)-1])
				changed = true
				break
			}
		}
	}

	return strings.Trim(strings.TrimSpace(name), " .,-@|!#")
}

func QueryClearbitCompanyDomain(companyName string) (string, error) {
	cleanName := SanitizeCompanyName(companyName)
	if cleanName == "" {
		cleanName = companyName
	}

	apiURL := fmt.Sprintf("https://autocomplete.clearbit.com/v1/companies/suggest?query=%s", url.QueryEscape(cleanName))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("autocomplete returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}

	var suggestions []struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
	}

	if err := json.Unmarshal(body, &suggestions); err != nil {
		return "", err
	}

	for _, s := range suggestions {
		dom := strings.ToLower(strings.TrimSpace(s.Domain))
		dom = strings.TrimPrefix(dom, "www.")
		if dom != "" && checkDomainMX(dom) {
			return dom, nil
		}
	}

	return "", fmt.Errorf("no valid domain found from autocomplete")
}

func ResolveCompanyDomain(companyName string) (string, error) {
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return "", fmt.Errorf("empty company name")
	}

	store := cache.GetGlobalStore()
	if cached, ok := store.GetCompanyDomain(companyName); ok && cached != "" {
		if checkDomainMX(cached) {
			return cached, nil
		}
	}

	sanitized := SanitizeCompanyName(companyName)
	if sanitized != "" && sanitized != companyName {
		if cached, ok := store.GetCompanyDomain(sanitized); ok && cached != "" {
			if checkDomainMX(cached) {
				return cached, nil
			}
		}
	}

	lower := strings.ToLower(companyName)
	if strings.Contains(lower, ".") && !strings.Contains(lower, " ") {
		clean := strings.Trim(lower, " ./@")
		if checkDomainMX(clean) {
			store.SetCompanyDomain(companyName, clean)
			return clean, nil
		}
	}

	// Clearbit Autocomplete Enrichment
	targetQuery := sanitized
	if targetQuery == "" {
		targetQuery = companyName
	}
	if dom, errAuto := QueryClearbitCompanyDomain(targetQuery); errAuto == nil && dom != "" {
		store.SetCompanyDomain(companyName, dom)
		if sanitized != "" {
			store.SetCompanyDomain(sanitized, dom)
		}
		return dom, nil
	}

	cleanedLower := strings.ToLower(targetQuery)
	var candidates []string

	if strings.HasSuffix(cleanedLower, " ai") {
		base := strings.TrimSpace(strings.TrimSuffix(cleanedLower, " ai"))
		reNonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
		slug := reNonAlpha.ReplaceAllString(base, "")
		if slug != "" {
			candidates = append(candidates, slug+".ai")
		}
	} else if strings.HasSuffix(cleanedLower, " io") {
		base := strings.TrimSpace(strings.TrimSuffix(cleanedLower, " io"))
		reNonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
		slug := reNonAlpha.ReplaceAllString(base, "")
		if slug != "" {
			candidates = append(candidates, slug+".io")
		}
	}

	reNonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
	fullSlug := reNonAlpha.ReplaceAllString(cleanedLower, "")
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

	words := strings.Fields(cleanedLower)
	if len(words) >= 2 {
		twoWords := reNonAlpha.ReplaceAllString(words[0]+words[1], "")
		if len(twoWords) >= 4 && len(twoWords) <= 25 {
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
			store.SetCompanyDomain(companyName, cand)
			if sanitized != "" {
				store.SetCompanyDomain(sanitized, cand)
			}
			return cand, nil
		}
	}

	return "", fmt.Errorf("could not resolve domain for %s", companyName)
}
