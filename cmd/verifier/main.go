package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"email-verifier-cli/internal/cache"
	"email-verifier-cli/internal/config"
	"email-verifier-cli/internal/csv"
	"email-verifier-cli/internal/excel"
	"email-verifier-cli/internal/linkedin"
	"email-verifier-cli/internal/models"
	"email-verifier-cli/internal/permutator"
	"email-verifier-cli/internal/scoring"
	"email-verifier-cli/internal/server"
	"email-verifier-cli/internal/smtp"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		port := 9876
		if err := server.StartSetupServer(port); err != nil {
			log.Fatalf("[ERROR] Setup server failed: %v", err)
		}
		return
	}

	setupFlag := flag.Bool("setup", false, "Launch the browser-based configuration setup")
	syncDaemon := flag.Bool("daemon", false, "Run persistent token sync daemon in background")
	syncPort := flag.Int("port", 9876, "Port for token sync daemon")

	fetchConnections := flag.String("fetch-connections", "", "Target LinkedIn Profile URL to fetch connection list for")
	fecthConnections := flag.String("fecth-connections", "", "Alias for -fetch-connections")
	limit := flag.Int("limit", 0, "Maximum number of connections or leads to fetch (default: 0 = fetch all)")
	liAt := flag.String("li-at", "", "LinkedIn li_at session cookie token")
	liRm := flag.String("li-rm", "", "LinkedIn li_rm persistent refresh cookie token")
	jSessionID := flag.String("jsessionid", "", "LinkedIn JSESSIONID cookie value")
	cookieHeader := flag.String("cookie", "", "Full raw Cookie header from browser")
	refreshSession := flag.Bool("refresh-session", false, "Force refresh li_at session cookie using stored li_rm")
	autoYes := flag.Bool("yes", false, "Automatically proceed with finding email addresses without prompting")
	autoYesShort := flag.Bool("y", false, "Short alias for -yes")
	importExport := flag.String("import-export", "", "Path to LinkedIn data export zip archive or Connections.csv")

	inputPath := flag.String("input", "contacts.xlsx", "Path to input Excel spreadsheet or CSV/ZIP")
	outputPath := flag.String("output", "verified_campaign.xlsx", "Path to output Excel spreadsheet")
	senderDomain := flag.String("sender-domain", "example.com", "Domain to use for HELO and MAIL FROM")
	timeoutSec := flag.Int("timeout", 5, "Connection and read timeout in seconds")
	delayMs := flag.Int("delay", 250, "Throttle delay in milliseconds between mailbox probes")
	concurrency := flag.Int("concurrency", 5, "Number of concurrent workers for email verification")
	searchQuery := flag.String("search", "", "Search LinkedIn for prospective leads by keyword or title")
	flag.Parse()

	if *setupFlag {
		if err := server.StartSetupServer(*syncPort); err != nil {
			log.Fatalf("[ERROR] Setup server failed: %v", err)
		}
		return
	}

	timeout := time.Duration(*timeoutSec) * time.Second
	throttleDelay := time.Duration(*delayMs) * time.Millisecond

	fmt.Println("==================================================")
	fmt.Println("             Email Permutation & Verifier         ")
	fmt.Println("==================================================")

	if *syncDaemon {
		if err := server.StartTokenSyncServer(*syncPort); err != nil {
			log.Fatalf("[ERROR] Daemon error: %v", err)
		}
		return
	}

	cfg, _ := config.LoadConfig()

	if *liAt != "" {
		cfg.LiAt = config.CleanCookieValue(*liAt)
	}
	if *liRm != "" {
		cfg.LiRm = config.CleanCookieValue(*liRm)
	}
	if *jSessionID != "" {
		cfg.JSessionID = config.CleanCookieValue(*jSessionID)
	}
	if *cookieHeader != "" {
		cfg.CookieHeader = strings.TrimSpace(*cookieHeader)
	}

	if *liAt != "" || *liRm != "" || *jSessionID != "" || *cookieHeader != "" {
		if err := config.SaveConfig(cfg); err == nil {
			fmt.Printf("[INFO] Saved LinkedIn session tokens to %s\n", config.GetConfigPath())
		}
	}

	if *refreshSession {
		client := linkedin.NewAPIClient(cfg.LiAt, cfg.LiRm, cfg.JSessionID, cfg.CookieHeader)
		if _, err := client.RefreshSessionWithLiRm(); err != nil {
			log.Fatalf("[ERROR] Session refresh failed: %v", err)
		}
		return
	}

	targetConnURL := *fetchConnections
	if targetConnURL == "" {
		targetConnURL = *fecthConnections
	}
	if targetConnURL != "" {
		handleFetchConnections(targetConnURL, *limit, *senderDomain, timeout, throttleDelay, *autoYes || *autoYesShort, *concurrency)
		return
	}

	if *searchQuery != "" {
		handleLinkedInSearch(*searchQuery, *limit, *senderDomain, timeout, throttleDelay, *autoYes || *autoYesShort, *concurrency)
		return
	}

	if *importExport != "" {
		handleLinkedInExport(*importExport, *senderDomain, timeout, throttleDelay, *autoYes || *autoYesShort, *concurrency)
		return
	}

	singleURL := flag.Lookup("url")
	singleDomain := flag.Lookup("domain")
	if singleURL != nil && singleURL.Value.String() != "" {
		handleSingleProfile(singleURL.Value.String(), singleDomain.Value.String(), *outputPath, *senderDomain, timeout, throttleDelay)
		return
	}

	handleExcelBatch(*inputPath, *outputPath, *senderDomain, timeout, throttleDelay, *concurrency)
}

func handleFetchConnections(targetProfileURL string, limit int, senderDomain string, timeout, throttleDelay time.Duration, autoProceed bool, concurrency int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("[WARN] Unable to load config file: %v\n", err)
	}

	info, err := linkedin.ParseProfileURL(targetProfileURL)
	if err != nil {
		fmt.Printf("[ERROR] Invalid LinkedIn URL: %v\n", err)
		os.Exit(1)
	}

	if cfg.LiAt == "" && cfg.LiRm == "" && cfg.CookieHeader == "" {
		fmt.Println("[ERROR] LinkedIn session credentials are not configured.")
		fmt.Println("[INFO] Run 'email-verifier setup' to configure your session.")
		os.Exit(1)
	}

	apiClient := linkedin.NewAPIClient(cfg.LiAt, cfg.LiRm, cfg.JSessionID, cfg.CookieHeader)

	fmt.Printf("[INFO] Authenticating with LinkedIn API using session cookies...\n")
	if limit > 0 {
		fmt.Printf("[INFO] Fetching up to %d connections for profile: %s %s (%s)...\n", limit, info.FirstName, info.LastName, info.URL)
	} else {
		fmt.Printf("[INFO] Fetching ALL connections for profile: %s %s (%s)...\n", info.FirstName, info.LastName, info.URL)
	}
	_, contacts, err := apiClient.FetchProfileConnections(targetProfileURL, limit)
	if err != nil {
		fmt.Printf("[ERROR] Failed to fetch connections: %v\n", err)
		os.Exit(1)
	}

	if len(contacts) == 0 {
		fmt.Printf("[WARN] No connections found for this profile.\n")
		return
	}

	fmt.Printf("[SUCCESS] Successfully retrieved %d connection records!\n", len(contacts))

	baseSlug := info.Slug
	if baseSlug == "" {
		baseSlug = strings.ToLower(fmt.Sprintf("%s_%s", info.FirstName, info.LastName))
	}
	csvFileName := fmt.Sprintf("%s_connections.csv", baseSlug)
	excelFileName := fmt.Sprintf("%s_verified_contacts.xlsx", baseSlug)

	csvPath, err := csv.SaveConnectionsToCSV(csvFileName, contacts)
	if err != nil {
		fmt.Printf("[ERROR] Error saving CSV file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n[SUCCESS] Saved connection list to CSV: %s\n", csvPath)

	if !autoProceed {
		fmt.Println("\n--------------------------------------------------")
		fmt.Printf("Do you want to proceed with finding email addresses for all %d fetched connections? (y/N): ", len(contacts))

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.ToLower(strings.TrimSpace(input))

		if input != "y" && input != "yes" {
			fmt.Println("[INFO] Email verification skipped. You can process the CSV anytime using -input.")
			return
		}
	}

	fmt.Println("\n==================================================")
	fmt.Println("   Starting Email Verification for Connections   ")
	fmt.Println("==================================================")

	verifiedContacts := verifyConnectionsList(contacts, senderDomain, timeout, throttleDelay, concurrency)

	excelPath, err := saveContactsToExcel(excelFileName, verifiedContacts)
	if err != nil {
		fmt.Printf("[ERROR] Failed to save verified contacts Excel workbook: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n==================================================")
	fmt.Printf("[SUCCESS] Finished! Verified results saved to Excel: %s\n", excelPath)
	fmt.Println("==================================================")
}

func handleLinkedInSearch(keywords string, limit int, senderDomain string, timeout, throttleDelay time.Duration, autoProceed bool, concurrency int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("[WARN] Unable to load config file: %v\n", err)
	}

	if cfg.LiAt == "" && cfg.LiRm == "" && cfg.CookieHeader == "" {
		fmt.Println("[ERROR] LinkedIn session credentials are not configured.")
		fmt.Println("[INFO] Run 'email-verifier setup' to configure your session.")
		os.Exit(1)
	}

	apiClient := linkedin.NewAPIClient(cfg.LiAt, cfg.LiRm, cfg.JSessionID, cfg.CookieHeader)

	fmt.Printf("[INFO] Authenticating with LinkedIn API...\n")
	fmt.Printf("[INFO] Searching LinkedIn for prospective leads: '%s' (limit: %d)...\n", keywords, limit)
	contacts, err := apiClient.SearchLeads(keywords, limit)
	if err != nil {
		fmt.Printf("[ERROR] Failed to search leads: %v\n", err)
		os.Exit(1)
	}

	if len(contacts) == 0 {
		fmt.Printf("[WARN] No prospective leads found for '%s'.\n", keywords)
		return
	}

	fmt.Printf("[SUCCESS] Successfully discovered %d prospective leads!\n", len(contacts))

	baseSlug := strings.ToLower(keywords)
	baseSlug = strings.ReplaceAll(baseSlug, " ", "_")
	baseSlug = strings.ReplaceAll(baseSlug, "/", "_")
	baseSlug = strings.ReplaceAll(baseSlug, "\"", "")
	baseSlug = strings.ReplaceAll(baseSlug, "'", "")
	csvFileName := fmt.Sprintf("%s_leads.csv", baseSlug)
	excelFileName := fmt.Sprintf("%s_verified_leads.xlsx", baseSlug)

	csvPath, err := csv.SaveConnectionsToCSV(csvFileName, contacts)
	if err != nil {
		fmt.Printf("[ERROR] Error saving CSV file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n[SUCCESS] Saved discovered leads to CSV: %s\n", csvPath)

	if !autoProceed {
		fmt.Println("\n--------------------------------------------------")
		fmt.Printf("Do you want to proceed with finding email addresses for all %d leads? (y/N): ", len(contacts))

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.ToLower(strings.TrimSpace(input))

		if input != "y" && input != "yes" {
			fmt.Println("[INFO] Email verification skipped. You can process the CSV anytime using -input.")
			return
		}
	}

	fmt.Println("\n==================================================")
	fmt.Println("       Starting Email Verification for Leads      ")
	fmt.Println("==================================================")

	verifiedContacts := verifyConnectionsList(contacts, senderDomain, timeout, throttleDelay, concurrency)

	excelPath, err := saveContactsToExcel(excelFileName, verifiedContacts)
	if err != nil {
		fmt.Printf("[ERROR] Failed to save verified contacts Excel workbook: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n==================================================")
	fmt.Printf("[SUCCESS] Finished! Verified results saved to Excel: %s\n", excelPath)
	fmt.Println("==================================================")
}

func handleLinkedInExport(exportPath, senderDomain string, timeout, throttleDelay time.Duration, autoProceed bool, concurrency int) {
	fmt.Printf("[INFO] Inspecting LinkedIn Data Export: %s\n", exportPath)

	contacts, suggestedName, err := linkedin.ParseLinkedInExport(exportPath)
	if err != nil {
		fmt.Printf("[ERROR] Failed to parse LinkedIn export: %v\n", err)
		os.Exit(1)
	}

	if len(contacts) == 0 {
		fmt.Println("[ERROR] No connection records found in the provided export archive.")
		fmt.Println("       Tip: Ensure your LinkedIn export contains 'Connections.csv'.")
		os.Exit(1)
	}

	cleanName := strings.ToLower(suggestedName)
	cleanName = strings.ReplaceAll(cleanName, " ", "_")
	cleanName = strings.ReplaceAll(cleanName, ".", "_")
	csvFileName := fmt.Sprintf("%s_connections.csv", cleanName)
	excelFileName := fmt.Sprintf("%s_verified_contacts.xlsx", cleanName)

	csvPath, err := csv.SaveConnectionsToCSV(csvFileName, contacts)
	if err != nil {
		fmt.Printf("[ERROR] Error saving CSV file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n[SUCCESS] Successfully extracted %d connection records!\n", len(contacts))
	fmt.Printf("[SUCCESS] Saved connection list to CSV: %s\n", csvPath)

	if !autoProceed {
		fmt.Println("\n--------------------------------------------------")
		fmt.Printf("Do you want to proceed with finding email addresses for all %d connections? (y/N): ", len(contacts))

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.ToLower(strings.TrimSpace(input))

		if input != "y" && input != "yes" {
			fmt.Println("[INFO] Email verification skipped. You can process the CSV anytime using -input.")
			return
		}
	}

	fmt.Println("\n==================================================")
	fmt.Println("   Starting Email Verification for Connections   ")
	fmt.Println("==================================================")

	verifiedContacts := verifyConnectionsList(contacts, senderDomain, timeout, throttleDelay, concurrency)

	excelPath, err := saveContactsToExcel(excelFileName, verifiedContacts)
	if err != nil {
		fmt.Printf("[ERROR] Failed to save verified contacts Excel workbook: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n==================================================")
	fmt.Printf("[SUCCESS] Finished! Verified results saved to Excel: %s\n", excelPath)
	fmt.Println("==================================================")
}

type DomainLockPool struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (p *DomainLockPool) GetLock(domain string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.locks == nil {
		p.locks = make(map[string]*sync.Mutex)
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if l, ok := p.locks[domain]; ok {
		return l
	}
	l := &sync.Mutex{}
	p.locks[domain] = l
	return l
}

func verifySingleContact(c models.Contact, senderDomain string, timeout, throttleDelay time.Duration, domainPool *DomainLockPool, verbose bool) models.Contact {
	firstName := c.FirstName
	lastName := c.LastName

	if firstName == "" {
		c.Status = "Skipped (Missing Name)"
		c.CampaignSent = "No"
		c.ConfidenceScore = 0
		c.ConfidenceTier = scoring.TierUndeliverable
		c.VerificationReason = "Skipped (Missing Name)"
		return c
	}

	personalDomains := []string{"gmail.com", "outlook.com"}
	for _, pDom := range personalDomains {
		var pLock *sync.Mutex
		if domainPool != nil {
			pLock = domainPool.GetLock(pDom)
			pLock.Lock()
		}

		pMXHosts, err := smtp.ResolveMXRecords(pDom)
		if err == nil && len(pMXHosts) > 0 {
			pCandidates := linkedin.GetCandidatePermutations(firstName, lastName, pDom)
			for _, cand := range pCandidates {
				if verbose {
					fmt.Printf("  -> Probing personal: %s ... ", cand)
				}
				if smtp.ProbeMailboxMultiMX(pMXHosts, cand, senderDomain, timeout) {
					if verbose {
						fmt.Println("[VALID]")
					}
					c.PersonalEmail = cand
					break
				}
				if verbose {
					fmt.Println("[REJECTED]")
				}
				time.Sleep(throttleDelay)
			}
		}

		if pLock != nil {
			pLock.Unlock()
		}

		if c.PersonalEmail != "" {
			break
		}
	}

	targetDomain := c.Domain
	if targetDomain == "gmail.com" || targetDomain == "outlook.com" || len(targetDomain) > 30 {
		targetDomain = ""
	}

	if targetDomain != "" {
		if hosts, err := smtp.ResolveMXRecords(targetDomain); err != nil || len(hosts) == 0 {
			targetDomain = ""
		}
	}

	if targetDomain == "" {
		if c.Status != "" {
			if comp := linkedin.ExtractCompanyFromHeadline(c.Status); comp != "" {
				if d, errD := linkedin.ResolveCompanyDomain(comp); errD == nil && d != "" {
					targetDomain = d
				}
			}
		}
		if targetDomain == "" && c.LinkedInURL != "" {
			if verbose {
				fmt.Printf("[INFO] Scraping LinkedIn profile for company domain: %s\n", c.LinkedInURL)
			}
			meta, err := linkedin.ScrapeProfile(c.LinkedInURL)
			if err == nil && meta != nil {
				if meta.Domain != "" {
					targetDomain = meta.Domain
				} else if meta.Company != "" {
					if d, errD := linkedin.ResolveCompanyDomain(meta.Company); errD == nil && d != "" {
						targetDomain = d
					}
				}
			}
		}
	}

	c.Domain = targetDomain

	isCatchAll := false
	vrfyConfirmed := false

	if targetDomain != "" && targetDomain != "gmail.com" && targetDomain != "outlook.com" {
		var lock *sync.Mutex
		if domainPool != nil {
			lock = domainPool.GetLock(targetDomain)
			lock.Lock()
		}

		mxHosts, err := smtp.ResolveMXRecords(targetDomain)
		if err == nil && len(mxHosts) > 0 {
			if !smtp.IsCatchAllMultiMX(mxHosts, targetDomain, senderDomain, timeout) {
				candidates := linkedin.GetCandidatePermutations(firstName, lastName, targetDomain)
				for _, candidate := range candidates {
					if verbose {
						fmt.Printf("  -> Probing company: %s ... ", candidate)
					}
					if smtp.ProbeMailboxMultiMX(mxHosts, candidate, senderDomain, timeout) {
						if verbose {
							fmt.Println("[VALID]")
						}
						c.WorkEmail = candidate
						// Store successful pattern in persistent memory
						pat := permutator.DetectPattern(candidate, firstName, lastName)
						cache.GetGlobalStore().SetDomainPattern(targetDomain, pat)
						break
					}
					if verbose {
						fmt.Println("[REJECTED]")
					}
					time.Sleep(throttleDelay)
				}
			} else {
				isCatchAll = true
				if verbose {
					fmt.Printf("  [WARN] Company domain %s is Catch-All (Applying Accept-All Strategy)...\n", targetDomain)
				}

				// 1. Check if domain pattern is cached
				var chosenCandidate string
				cachedPat, hasPat := cache.GetGlobalStore().GetDomainPattern(targetDomain)
				if hasPat && cachedPat != "" {
					chosenCandidate = permutator.FormatPattern(cachedPat, firstName, lastName, targetDomain)
				} else {
					chosenCandidate = permutator.GetMostLikelyCandidate(firstName, lastName, targetDomain)
				}

				// 2. Check VRFY probe if supported
				if len(mxHosts) > 0 {
					vrfyValid, vrfySupported := smtp.ProbeVRFY(mxHosts[0], chosenCandidate, senderDomain, timeout)
					if vrfySupported && vrfyValid {
						vrfyConfirmed = true
						if verbose {
							fmt.Printf("  [VRFY-CONFIRMED] %s\n", chosenCandidate)
						}
					}
				}

				c.WorkEmail = chosenCandidate
			}
		}

		if lock != nil {
			lock.Unlock()
		}
	}

	if c.PersonalEmail != "" {
		c.VerifiedEmail = c.PersonalEmail
		if c.WorkEmail != "" {
			if isCatchAll {
				if vrfyConfirmed {
					c.Status = "Verified (Personal) + Work (VRFY Confirmed)"
				} else {
					c.Status = "Verified (Personal) + Work (Catch-All)"
				}
			} else {
				c.Status = "Verified (Personal + Work)"
			}
		} else {
			c.Status = "Verified (Personal)"
		}
		c.CampaignSent = "Pending"
	} else if c.WorkEmail != "" {
		c.VerifiedEmail = c.WorkEmail
		if isCatchAll {
			if vrfyConfirmed {
				c.Status = "Verified (Work - VRFY Confirmed)"
			} else {
				c.Status = "Catch-All (High Confidence)"
			}
		} else {
			c.Status = "Verified (Work)"
		}
		c.CampaignSent = "Pending"
	} else {
		c.Status = "Not Found"
		c.CampaignSent = "No"
	}

	patternMatched := false
	if isCatchAll && targetDomain != "" {
		_, patternMatched = cache.GetGlobalStore().GetDomainPattern(targetDomain)
	}

	hasValidMX := targetDomain != "" || c.PersonalEmail != ""
	if targetDomain != "" {
		if hosts, err := smtp.ResolveMXRecords(targetDomain); err == nil && len(hosts) > 0 {
			hasValidMX = true
		}
	}

	scoreRes := scoring.EvaluateScore(
		c.PersonalEmail != "",
		c.WorkEmail != "",
		isCatchAll,
		vrfyConfirmed,
		patternMatched,
		hasValidMX,
		false,
	)

	c.ConfidenceScore = scoreRes.Score
	c.ConfidenceTier = scoreRes.Tier
	c.VerificationReason = scoreRes.Reason

	return c
}

func verifyConnectionsList(contacts []models.Contact, senderDomain string, timeout, throttleDelay time.Duration, concurrency int) []models.Contact {
	total := len(contacts)
	if total == 0 {
		return contacts
	}

	if concurrency <= 0 {
		concurrency = 5
	}
	if concurrency > total {
		concurrency = total
	}

	results := make([]models.Contact, total)

	if concurrency == 1 {
		for i, c := range contacts {
			fmt.Printf("\n[%d/%d] Processing Connection: %s %s\n", i+1, total, c.FirstName, c.LastName)
			res := verifySingleContact(c, senderDomain, timeout, throttleDelay, nil, true)
			results[i] = res
			if res.VerifiedEmail != "" {
				fmt.Printf("  [%d%% %s] %s (%s)\n", res.ConfidenceScore, res.ConfidenceTier, res.VerifiedEmail, res.VerificationReason)
			} else {
				fmt.Printf("  [0%% %s] %s (%s)\n", res.ConfidenceTier, res.Status, res.VerificationReason)
			}
		}
		return results
	}

	fmt.Printf("[INFO] Launching %d concurrent verification workers for %d contacts...\n\n", concurrency, total)

	domainPool := &DomainLockPool{locks: make(map[string]*sync.Mutex)}
	jobs := make(chan int, total)
	for i := 0; i < total; i++ {
		jobs <- i
	}
	close(jobs)

	var completedCounter int32
	var printMu sync.Mutex
	var wg sync.WaitGroup

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				c := contacts[idx]
				res := verifySingleContact(c, senderDomain, timeout, throttleDelay, domainPool, false)
				results[idx] = res

				curr := atomic.AddInt32(&completedCounter, 1)
				printMu.Lock()
				badge := fmt.Sprintf("[%d%% %s]", res.ConfidenceScore, res.ConfidenceTier)
				if res.PersonalEmail != "" && res.WorkEmail != "" {
					fmt.Printf("[%d/%d] %s %s %s -> Personal: %s | Work: %s (%s)\n", curr, total, badge, res.FirstName, res.LastName, res.PersonalEmail, res.WorkEmail, res.VerificationReason)
				} else if res.PersonalEmail != "" {
					fmt.Printf("[%d/%d] %s %s %s -> %s (%s)\n", curr, total, badge, res.FirstName, res.LastName, res.PersonalEmail, res.VerificationReason)
				} else if res.WorkEmail != "" {
					fmt.Printf("[%d/%d] %s %s %s -> %s (%s)\n", curr, total, badge, res.FirstName, res.LastName, res.WorkEmail, res.VerificationReason)
				} else {
					fmt.Printf("[%d/%d] %s %s %s -> %s (%s)\n", curr, total, badge, res.FirstName, res.LastName, res.Status, res.VerificationReason)
				}
				printMu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results
}

func saveContactsToExcel(outputPath string, contacts []models.Contact) (string, error) {
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		absPath = outputPath
	}
	if err := excel.CreateSpreadsheet(absPath, contacts); err != nil {
		return "", err
	}
	return absPath, nil
}

func handleSingleProfile(urlStr, domain, outputPath, senderDomain string, timeout, throttleDelay time.Duration) {
	fmt.Printf("[INFO] Scraping single profile: %s\n", urlStr)
	meta, err := linkedin.ScrapeProfile(urlStr)
	if err != nil {
		log.Fatalf("[ERROR] Profile scrape failed: %v", err)
	}

	if meta.FirstName == "" && meta.LastName == "" {
		log.Fatalf("[ERROR] Could not extract name from profile %s", urlStr)
	}

	targetDomain := domain
	if targetDomain == "" {
		if meta.Domain != "" {
			targetDomain = meta.Domain
		} else if meta.Company != "" {
			fmt.Printf("[INFO] Resolving corporate domain for '%s'...\n", meta.Company)
			resolvedDomain, errDomain := linkedin.ResolveCompanyDomain(meta.Company)
			if errDomain == nil && resolvedDomain != "" {
				targetDomain = resolvedDomain
			}
		}
	}

	if targetDomain == "" {
		log.Fatalf("[ERROR] Target corporate domain could not be resolved. Please specify manually using -domain <domain.com>.")
	}

	fmt.Printf("[SUCCESS] Target Contact: %s %s (@%s)\n", meta.FirstName, meta.LastName, targetDomain)
	c := models.Contact{
		FirstName:   meta.FirstName,
		LastName:    meta.LastName,
		Domain:      targetDomain,
		LinkedInURL: urlStr,
	}

	verified := verifySingleContact(c, senderDomain, timeout, throttleDelay, nil, true)
	if verified.VerifiedEmail != "" {
		fmt.Printf("\n[VALID] Found valid mailbox: %s (Confidence: %d%% - %s)\n", verified.VerifiedEmail, verified.ConfidenceScore, verified.ConfidenceTier)
	} else {
		fmt.Printf("\n[INFO] No valid corporate mailbox verified (%d%% - %s: %s).\n", verified.ConfidenceScore, verified.ConfidenceTier, verified.VerificationReason)
	}

	if errSave := excel.CreateSpreadsheet(outputPath, []models.Contact{verified}); errSave != nil {
		log.Fatalf("[ERROR] Failed to create output spreadsheet: %v", errSave)
	}

	fmt.Printf("[SUCCESS] Output saved to: %s\n", outputPath)
}

func handleExcelBatch(inputPath, outputPath, senderDomain string, timeout, throttleDelay time.Duration, concurrency int) {
	ext := strings.ToLower(filepath.Ext(inputPath))

	var contacts []models.Contact
	var handler *excel.Handler
	var err error

	if ext == ".zip" || (ext == ".csv" && (strings.Contains(strings.ToLower(inputPath), "connections") || strings.Contains(strings.ToLower(inputPath), "export"))) {
		parsedContacts, suggestedName, parseErr := linkedin.ParseLinkedInExport(inputPath)
		if parseErr != nil {
			log.Fatalf("[ERROR] Failed to parse input export: %v", parseErr)
		}
		contacts = parsedContacts
		if outputPath == "verified_campaign.xlsx" && suggestedName != "" {
			cleanName := strings.ToLower(suggestedName)
			cleanName = strings.ReplaceAll(cleanName, " ", "_")
			cleanName = strings.ReplaceAll(cleanName, ".", "_")
			outputPath = fmt.Sprintf("%s_verified_contacts.xlsx", cleanName)
		}
	} else if ext == ".csv" {
		contacts, err = csv.ReadContactsFromCSV(inputPath)
		if err != nil {
			log.Fatalf("[ERROR] Failed to open CSV file: %v", err)
		}
	} else {
		handler, contacts, err = excel.OpenSpreadsheet(inputPath)
		if err != nil {
			log.Fatalf("[ERROR] Failed to open Excel file: %v", err)
		}
		defer handler.Close()
	}

	total := len(contacts)
	fmt.Printf("[INFO] Loaded %d contact records from %s.\n\n", total, inputPath)

	verified := verifyConnectionsList(contacts, senderDomain, timeout, throttleDelay, concurrency)

	if handler != nil {
		for _, c := range verified {
			_ = handler.UpdateContactRow(c)
		}
		if err := handler.SaveAs(outputPath); err != nil {
			log.Fatalf("[ERROR] Failed to save output workbook: %v", err)
		}
	} else {
		if err := excel.CreateSpreadsheet(outputPath, verified); err != nil {
			log.Fatalf("[ERROR] Failed to save output workbook: %v", err)
		}
	}

	fmt.Println("\n==================================================")
	fmt.Printf("[SUCCESS] Finished. Results saved to: %s\n", outputPath)
	fmt.Println("==================================================")
}
