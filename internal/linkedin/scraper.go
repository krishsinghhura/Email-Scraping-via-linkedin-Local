package linkedin

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ProfileMetadata struct {
	FullName  string
	FirstName string
	LastName  string
	Headline  string
	Company   string
	Domain    string
}

func ScrapeProfile(rawURL string) (*ProfileMetadata, error) {
	formattedURL := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(formattedURL, "http://") && !strings.HasPrefix(formattedURL, "https://") {
		formattedURL = "https://" + formattedURL
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", formattedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	body := string(bodyBytes)
	meta := parseMetadataFromHTML(body)
	if meta.FullName == "" && meta.Company == "" {
		return nil, fmt.Errorf("could not extract profile name or company from page HTML")
	}

	return meta, nil
}

func parseMetadataFromHTML(htmlContent string) *ProfileMetadata {
	meta := &ProfileMetadata{}

	reOGTitle := regexp.MustCompile(`(?i)<meta\s+property=["']og:title["']\s+content=["']([^"']+)["']`)
	reTitle := regexp.MustCompile(`(?i)<title>([^<]+)</title>`)

	var pageTitle string
	if m := reOGTitle.FindStringSubmatch(htmlContent); len(m) > 1 {
		pageTitle = m[1]
	} else if m := reTitle.FindStringSubmatch(htmlContent); len(m) > 1 {
		pageTitle = m[1]
	}

	if pageTitle != "" {
		parseTitleString(pageTitle, meta)
	}

	reOGDesc := regexp.MustCompile(`(?i)<meta\s+(?:property|name)=["'](?:og:description|description)["']\s+content=["']([^"']+)["']`)
	if m := reOGDesc.FindStringSubmatch(htmlContent); len(m) > 1 {
		desc := m[1]
		if meta.Headline == "" {
			meta.Headline = desc
		}
		if meta.Company == "" {
			meta.Company = extractCompanyFromDescription(desc)
		}
	}

	return meta
}

func parseTitleString(title string, meta *ProfileMetadata) {
	title = regexp.MustCompile(`(?i)\s*[-|]\s*LinkedIn\s*$`).ReplaceAllString(title, "")

	parts := regexp.MustCompile(`\s*[-|–—:]\s*`).Split(title, -1)
	if len(parts) > 0 {
		rawName := strings.TrimSpace(parts[0])
		if rawName != "" {
			meta.FullName = rawName
			nameParts := strings.Fields(rawName)
			if len(nameParts) >= 1 {
				meta.FirstName = nameParts[0]
			}
			if len(nameParts) >= 2 {
				meta.LastName = strings.Join(nameParts[1:], " ")
			}
		}
	}

	if len(parts) >= 2 && meta.Headline == "" {
		meta.Headline = strings.TrimSpace(parts[1])
	}

	if len(parts) >= 3 && meta.Company == "" {
		meta.Company = cleanCompanyName(parts[len(parts)-1])
	} else if len(parts) == 2 && meta.Company == "" {
		meta.Company = cleanCompanyName(parts[1])
	}
}

func extractCompanyFromDescription(desc string) string {
	reAt := regexp.MustCompile(`(?i)\b(?:at|@)\s+([A-Z0-9\s&.,'-]+?)(?:\s+in|\s+·|\.|,|;|\)|$)`)
	if m := reAt.FindStringSubmatch(desc); len(m) > 1 {
		comp := cleanCompanyName(m[1])
		if comp != "" && len(comp) < 40 {
			return comp
		}
	}

	reWorks := regexp.MustCompile(`(?i)\bworks?\s+at\s+([A-Z0-9\s&.,'-]+?)(?:\s+in|\s+·|\.|,|;|\)|$)`)
	if m := reWorks.FindStringSubmatch(desc); len(m) > 1 {
		comp := cleanCompanyName(m[1])
		if comp != "" && len(comp) < 40 {
			return comp
		}
	}

	return ""
}

func cleanCompanyName(raw string) string {
	comp := strings.TrimSpace(raw)
	reSuffix := regexp.MustCompile(`(?i)\s*(?:Inc\.?|LLC\.?|Pvt\.?\s*Ltd\.?|Ltd\.?|Corp\.?|Corporation)$`)
	comp = reSuffix.ReplaceAllString(comp, "")
	return strings.TrimSpace(comp)
}
