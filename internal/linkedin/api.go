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
	"email-verifier-cli/internal/config"
	"email-verifier-cli/internal/models"
)

type APIClient struct {
	LiAt         string
	LiRm         string
	JSessionID   string
	CookieHeader string
	HTTPClient   *http.Client
}

func NewAPIClient(liAt, liRm, jSessionID, cookieHeader string) *APIClient {
	liAt = strings.TrimSpace(liAt)
	liRm = strings.TrimSpace(liRm)
	jSessionID = strings.TrimSpace(jSessionID)
	cookieHeader = strings.TrimSpace(cookieHeader)

	if jSessionID == "" {
		if strings.Contains(cookieHeader, "JSESSIONID=") {
			parts := strings.Split(cookieHeader, "JSESSIONID=")
			if len(parts) > 1 {
				sub := strings.Split(parts[1], ";")[0]
				jSessionID = strings.Trim(sub, "\"")
			}
		}
		if jSessionID == "" {
			jSessionID = "ajax:1234567890123456789"
		}
	}

	return &APIClient{
		LiAt:         liAt,
		LiRm:         liRm,
		JSessionID:   jSessionID,
		CookieHeader: cookieHeader,
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 2 {
					return http.ErrUseLastResponse
				}
				if len(via) > 0 {
					for k, vv := range via[0].Header {
						if strings.EqualFold(k, "Cookie") {
							for _, v := range vv {
								req.Header.Add(k, v)
							}
						}
					}
				}
				return nil
			},
		},
	}
}

func (c *APIClient) newRequest(method, reqURL string) (*http.Request, error) {
	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return nil, err
	}

	csrfToken := strings.Trim(c.JSessionID, "\"")
	cookieHeader := c.CookieHeader
	if cookieHeader == "" {
		cookieHeader = fmt.Sprintf("li_at=%s; JSESSIONID=\"%s\"", c.LiAt, csrfToken)
		if c.LiRm != "" {
			cookieHeader = fmt.Sprintf("li_rm=%s; %s", c.LiRm, cookieHeader)
		}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("csrf-token", csrfToken)
	req.Header.Set("x-restli-protocol-version", "2.0.0")
	req.Header.Set("Accept", "application/vnd.linkedin.normalized+json+2.0, application/json")

	return req, nil
}

func (c *APIClient) RefreshSessionWithLiRm() (bool, error) {
	if c.LiRm == "" {
		return false, fmt.Errorf("no li_rm cookie configured for refresh")
	}

	fmt.Println("[INFO] Attempting to refresh session using persistent 'li_rm' cookie...")

	refreshReq, err := http.NewRequest("GET", "https://www.linkedin.com/feed/", nil)
	if err != nil {
		return false, err
	}

	cookieHeader := fmt.Sprintf("li_rm=%s; JSESSIONID=\"%s\"", c.LiRm, strings.Trim(c.JSessionID, "\""))
	if c.LiAt != "" {
		cookieHeader = fmt.Sprintf("li_at=%s; %s", c.LiAt, cookieHeader)
	}

	refreshReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	refreshReq.Header.Set("Cookie", cookieHeader)

	resp, err := c.HTTPClient.Do(refreshReq)
	if err != nil {
		return false, fmt.Errorf("failed to contact LinkedIn for session refresh: %w", err)
	}
	defer resp.Body.Close()

	var newLiAt string
	var newJSession string
	for _, ck := range resp.Cookies() {
		if ck.Name == "li_at" && ck.Value != "" {
			newLiAt = ck.Value
		}
		if ck.Name == "JSESSIONID" && ck.Value != "" {
			newJSession = ck.Value
		}
	}

	if newLiAt != "" {
		c.LiAt = newLiAt
		if newJSession != "" {
			c.JSessionID = newJSession
		}

		cfg, _ := config.LoadConfig()
		cfg.LiAt = newLiAt
		cfg.LiRm = c.LiRm
		if newJSession != "" {
			cfg.JSessionID = newJSession
		}
		_ = config.SaveConfig(cfg)

		fmt.Printf("[SUCCESS] Successfully refreshed session cookie!\n")
		fmt.Printf("[INFO] Updated configuration saved to %s\n", config.GetConfigPath())
		return true, nil
	}

	return false, fmt.Errorf("LinkedIn did not issue a new li_at cookie")
}

func (c *APIClient) FetchProfileConnections(profileURLOrSlug string, limit int) (*ProfileInfo, []models.Contact, error) {
	info, err := ParseProfileURL(profileURLOrSlug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse profile URL: %w", err)
	}

	fetchAll := false
	if limit <= 0 {
		fetchAll = true
		limit = 1000000
	}

	pageSize := 40
	allContacts := make([]models.Contact, 0)
	seen := make(map[string]bool)

	for start := 0; start < limit; start += pageSize {
		count := pageSize
		if !fetchAll && start+count > limit {
			count = limit - start
		}

		pageContacts, err := c.fetchConnectionsPage(start, count)
		if err != nil {
			if len(allContacts) == 0 {
				return info, nil, err
			}
			break
		}

		if len(pageContacts) == 0 {
			break
		}

		newFound := 0
		for _, contact := range pageContacts {
			key := contact.LinkedInURL
			if key == "" {
				key = fmt.Sprintf("%s_%s", contact.FirstName, contact.LastName)
			}
			if !seen[key] {
				seen[key] = true
				contact.RowIndex = len(allContacts) + 1
				allContacts = append(allContacts, contact)
				newFound++
			}
			if !fetchAll && len(allContacts) >= limit {
				break
			}
		}

		if newFound == 0 {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	for i := range allContacts {
		if allContacts[i].Domain == "" && allContacts[i].LastName != "" {
			if comp := extractCompanyFromHeadline(allContacts[i].Status); comp != "" {
				if dom, errDom := ResolveCompanyDomain(comp); errDom == nil {
					allContacts[i].Domain = dom
				}
			}
		}
	}

	return info, allContacts, nil
}

func (c *APIClient) fetchConnectionsPage(start, count int) ([]models.Contact, error) {
	apiURL := fmt.Sprintf("https://www.linkedin.com/voyager/api/relationships/connections?count=%d&start=%d", count, start)
	req, err := c.newRequest("GET", apiURL)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query LinkedIn API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read API response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if c.LiRm != "" {
			refreshed, refErr := c.RefreshSessionWithLiRm()
			if refreshed && refErr == nil {
				return c.fetchConnectionsPage(start, count)
			}
		}
		return nil, fmt.Errorf("LinkedIn API returned HTTP %d: Unauthorized", resp.StatusCode)
	}

	contacts := parseFlexibleConnectionsJSON(bodyBytes)
	if len(contacts) == 0 {
		searchContacts, errSearch := c.fetchSearchConnectionsPage(start, count)
		if errSearch == nil && len(searchContacts) > 0 {
			contacts = searchContacts
		}
	}

	return contacts, nil
}

func (c *APIClient) fetchSearchConnectionsPage(start, count int) ([]models.Contact, error) {
	searchURL := fmt.Sprintf("https://www.linkedin.com/voyager/api/search/dash/clusters?decorationId=com.linkedin.voyager.dash.deco.search.SearchClusterCollection-165&origin=MEMBER_PROFILE_CGI&q=all&query=(flagshipSearchIntent:SEARCH_MY_CONNECTIONS)&count=%d&start=%d", count, start)
	req, err := c.newRequest("GET", searchURL)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}

	return parseFlexibleConnectionsJSON(bodyBytes), nil
}

func parseFlexibleConnectionsJSON(body []byte) []models.Contact {
	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil
	}

	var contacts []models.Contact
	seen := make(map[string]bool)

	addContact := func(fn, ln, headline, slug, urlStr string) {
		fn = strings.TrimSpace(fn)
		ln = strings.TrimSpace(ln)
		if fn == "" && ln == "" {
			return
		}
		if urlStr == "" && slug != "" {
			urlStr = fmt.Sprintf("https://www.linkedin.com/in/%s", slug)
		}

		key := urlStr
		if key == "" {
			key = fmt.Sprintf("%s %s", fn, ln)
		}
		if seen[key] {
			return
		}
		seen[key] = true

		comp := extractCompanyFromHeadline(headline)
		dom := ""
		if comp != "" {
			if resolved, err := ResolveCompanyDomain(comp); err == nil {
				dom = resolved
			}
		}

		contacts = append(contacts, models.Contact{
			FirstName:   fn,
			LastName:    ln,
			Domain:      dom,
			LinkedInURL: urlStr,
			Status:      headline,
		})
	}

	getStringField := func(m map[string]interface{}, keys ...string) string {
		for _, k := range keys {
			if val, ok := m[k]; ok && val != nil {
				switch v := val.(type) {
				case string:
					if strings.TrimSpace(v) != "" {
						return strings.TrimSpace(v)
					}
				case map[string]interface{}:
					if txt, okTxt := v["text"].(string); okTxt && strings.TrimSpace(txt) != "" {
						return strings.TrimSpace(txt)
					}
				}
			}
		}
		return ""
	}

	if elements, ok := root["elements"].([]interface{}); ok {
		for _, item := range elements {
			if obj, isObj := item.(map[string]interface{}); isObj {
				if mp, okMP := obj["miniProfile"].(map[string]interface{}); okMP {
					fn := getStringField(mp, "firstName")
					ln := getStringField(mp, "lastName")
					headline := getStringField(mp, "headline", "occupation")
					slug := getStringField(mp, "publicIdentifier")
					addContact(fn, ln, headline, slug, "")
				}
			}
		}
	}

	if included, ok := root["included"].([]interface{}); ok {
		for _, item := range included {
			if obj, isObj := item.(map[string]interface{}); isObj {
				fn := getStringField(obj, "firstName")
				ln := getStringField(obj, "lastName")
				headline := getStringField(obj, "headline", "occupation", "primarySubtitle", "secondarySubtitle")
				slug := getStringField(obj, "publicIdentifier")
				navURL := getStringField(obj, "navigationUrl")

				title := getStringField(obj, "title")
				if (fn == "" || ln == "") && title != "" {
					parts := strings.Fields(title)
					if len(parts) >= 1 && fn == "" {
						fn = parts[0]
					}
					if len(parts) >= 2 && ln == "" {
						ln = strings.Join(parts[1:], " ")
					}
				}

				if fn != "" || ln != "" || slug != "" {
					addContact(fn, ln, headline, slug, navURL)
				}
			}
		}
	}

	return contacts
}

func ExtractCompanyFromHeadline(headline string) string {
	return extractCompanyFromHeadline(headline)
}

func extractCompanyFromHeadline(headline string) string {
	if headline == "" {
		return ""
	}

	lower := strings.ToLower(headline)
	var comp string

	ampIdx := strings.Index(headline, "@")
	atIdx := strings.Index(lower, " at ")

	if ampIdx != -1 {
		comp = headline[ampIdx+1:]
	} else if atIdx != -1 {
		comp = headline[atIdx+4:]
	}

	if comp == "" {
		reLead := regexp.MustCompile(`^(?:[A-Za-z\s/&-]+,\s*)([A-Za-z0-9\s&.-]+?)(?:\s*[|•·—–-]|\s*$)`)
		if m := reLead.FindStringSubmatch(headline); len(m) > 1 {
			words := strings.Fields(m[1])
			if len(words) <= 4 {
				comp = m[1]
			}
		}
	}

	if comp == "" {
		return ""
	}

	delims := []string{"||", "|", "•", "·", " - ", " – ", " — ", ",", ";", "(", "/", "\n", " ex-", " ex ", " ex.", " formerly "}
	for _, d := range delims {
		if dIdx := strings.Index(strings.ToLower(comp), d); dIdx != -1 {
			comp = comp[:dIdx]
		}
	}

	comp = regexp.MustCompile(`(?i)\b(?:by|with|in|for|and)\b.*$`).ReplaceAllString(comp, "")
	comp = cleanCompanyName(comp)
	comp = strings.Trim(comp, " .,-@|!#")

	words := strings.Fields(comp)
	if len(words) > 4 || len(comp) > 30 || len(comp) < 2 {
		return ""
	}
	return comp
}

func (c *APIClient) SearchLeads(keywords string, limit int) ([]models.Contact, error) {
	if limit <= 0 {
		limit = 20
	}

	var allContacts []models.Contact
	pageSize := 10
	if limit < pageSize {
		pageSize = limit
	}

	encodedKeywords := url.QueryEscape(keywords)
	seenURLs := make(map[string]bool)

	for start := 0; start < limit; start += pageSize {
		fetchCount := pageSize
		if start+fetchCount > limit {
			fetchCount = limit - start
		}

		apiURL := fmt.Sprintf(
			"https://www.linkedin.com/voyager/api/search/dash/clusters?decorationId=com.linkedin.voyager.dash.deco.search.SearchClusterCollection-175&origin=SWITCH_SEARCH_VERTICAL&q=all&query=(keywords:%s,flagshipSearchIntent:SEARCH_SRP,resultType:List(PEOPLE))&count=%d&start=%d",
			encodedKeywords, fetchCount, start,
		)

		req, err := c.newRequest("GET", apiURL)
		if err != nil {
			return allContacts, fmt.Errorf("failed to create search request: %w", err)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return allContacts, fmt.Errorf("search request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return allContacts, fmt.Errorf("search API returned status %d: %s", resp.StatusCode, string(body))
		}

		var searchResp struct {
			Elements []struct {
				Items []struct {
					ItemUnion struct {
						EntityResult struct {
							Title struct {
								Text string `json:"text"`
							} `json:"title"`
							PrimarySubtitle struct {
								Text string `json:"text"`
							} `json:"primarySubtitle"`
							SecondarySubtitle struct {
								Text string `json:"text"`
							} `json:"secondarySubtitle"`
							NavigationURL string `json:"navigationUrl"`
						} `json:"entityResult"`
					} `json:"itemUnion"`
				} `json:"items"`
			} `json:"elements"`
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(&searchResp)
		resp.Body.Close()
		if decodeErr != nil {
			return allContacts, fmt.Errorf("failed to decode search response: %w", decodeErr)
		}

		pageAdded := 0
		for _, el := range searchResp.Elements {
			for _, it := range el.Items {
				res := it.ItemUnion.EntityResult
				navURL := res.NavigationURL
				if navURL == "" || !strings.Contains(navURL, "/in/") {
					continue
				}

				cleanURL := strings.Split(navURL, "?")[0]
				if seenURLs[cleanURL] {
					continue
				}
				seenURLs[cleanURL] = true

				fullName := strings.TrimSpace(res.Title.Text)
				if fullName == "" {
					continue
				}

				parts := strings.Fields(fullName)
				firstName := parts[0]
				lastName := ""
				if len(parts) > 1 {
					lastName = strings.Join(parts[1:], " ")
				}

				headline := strings.TrimSpace(res.PrimarySubtitle.Text)

				contact := models.Contact{
					FirstName:   firstName,
					LastName:    lastName,
					LinkedInURL: cleanURL,
					Status:      headline,
					RowIndex:    len(allContacts) + 2,
				}

				if headline != "" {
					if comp := ExtractCompanyFromHeadline(headline); comp != "" {
						if dom, errDom := ResolveCompanyDomain(comp); errDom == nil && dom != "" {
							contact.Domain = dom
						}
					}
				}

				allContacts = append(allContacts, contact)
				pageAdded++
				if len(allContacts) >= limit {
					break
				}
			}
			if len(allContacts) >= limit {
				break
			}
		}

		if pageAdded == 0 {
			break
		}

		time.Sleep(300 * time.Millisecond)
	}

	return allContacts, nil
}

func (c *APIClient) FetchCompanyWebsite(companyUniversalNameOrID string) (string, error) {
	companyUniversalNameOrID = strings.TrimSpace(companyUniversalNameOrID)
	if companyUniversalNameOrID == "" {
		return "", fmt.Errorf("empty company identifier")
	}

	store := cache.GetGlobalStore()
	if dom, ok := store.GetCompanyDomain(companyUniversalNameOrID); ok && dom != "" {
		if checkDomainMX(dom) {
			return dom, nil
		}
	}

	if c.LiAt != "" || c.CookieHeader != "" {
		apiURL := fmt.Sprintf("https://www.linkedin.com/voyager/api/organization/companies?decorationId=com.linkedin.voyager.deco.organization.web.WebFullCompanyMain-34&q=universalName&universalName=%s", url.QueryEscape(companyUniversalNameOrID))
		req, err := c.newRequest("GET", apiURL)
		if err == nil {
			resp, errDo := c.HTTPClient.Do(req)
			if errDo == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
					var res struct {
						Elements []struct {
							WebsiteURL     string `json:"websiteUrl"`
							CompanyPageURL string `json:"companyPageUrl"`
						} `json:"elements"`
					}
					if errJSON := json.Unmarshal(body, &res); errJSON == nil && len(res.Elements) > 0 {
						website := res.Elements[0].WebsiteURL
						if website == "" {
							website = res.Elements[0].CompanyPageURL
						}
						if website != "" {
							dom := ExtractRootDomain(website)
							if dom != "" && checkDomainMX(dom) {
								store.SetCompanyDomain(companyUniversalNameOrID, dom)
								return dom, nil
							}
						}
					}
				}
			}
		}
	}

	pubURL := fmt.Sprintf("https://www.linkedin.com/company/%s/about/", companyUniversalNameOrID)
	req, err := http.NewRequest("GET", pubURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		resp, errDo := c.HTTPClient.Do(req)
		if errDo == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
				content := string(body)
				reSameAs := regexp.MustCompile(`"sameAs"\s*:\s*"([^"]+)"`)
				if m := reSameAs.FindStringSubmatch(content); len(m) > 1 {
					dom := ExtractRootDomain(m[1])
					if dom != "" && checkDomainMX(dom) {
						store.SetCompanyDomain(companyUniversalNameOrID, dom)
						return dom, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not fetch website for company %s", companyUniversalNameOrID)
}

