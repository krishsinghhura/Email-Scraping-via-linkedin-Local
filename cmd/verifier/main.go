package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"email-verifier-cli/internal/config"
	"email-verifier-cli/internal/csv"
	"email-verifier-cli/internal/excel"
	"email-verifier-cli/internal/linkedin"
	"email-verifier-cli/internal/models"
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

	linkedinURL := flag.String("url", "", "Target LinkedIn Profile URL for direct lookup")
	targetDomain := flag.String("domain", "", "Target corporate domain")
	setupFlag := flag.Bool("setup", false, "Start browser-based configuration setup")
	syncDaemon := flag.Bool("daemon", false, "Start background HTTP token sync daemon on localhost:9876")
	syncPort := flag.Int("port", 9876, "Port for token sync daemon")

	fetchConnections := flag.String("fetch-connections", "", "Target LinkedIn Profile URL to fetch connection list for")
	fecthConnections := flag.String("fecth-connections", "", "Alias for -fetch-connections")
	limit := flag.Int("limit", 100, "Maximum number of connections to fetch")
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
		handleFetchConnections(targetConnURL, *limit, cfg, *senderDomain, timeout, throttleDelay, *autoYes || *autoYesShort)
		return
	}

	if *importExport != "" {
		handleLinkedInExport(*importExport, *senderDomain, timeout, throttleDelay, *autoYes || *autoYesShort)
		return
	}

	if *linkedinURL != "" {
		handleLinkedInLookup(*linkedinURL, *targetDomain, *senderDomain, timeout, throttleDelay)
		return
	}

	handleExcelBatch(*inputPath, *outputPath, *senderDomain, timeout, throttleDelay)
}

func handleFetchConnections(targetProfileURL string, limit int, cfg *config.Config, senderDomain string, timeout, throttleDelay time.Duration, autoProceed bool) {
	info, err := linkedin.ParseProfileURL(targetProfileURL)
	if err != nil {
		fmt.Printf("[ERROR] Invalid LinkedIn URL: %v\n", err)
		os.Exit(1)
	}

	if cfg.LiAt == "" && cfg.LiRm == "" && cfg.CookieHeader == "" {
		fmt.Println("[ERROR] LinkedIn session credentials are not configured.")
		fmt.Println("[INFO] Run 'email-verifier setup' to configure your session in the browser.")
		os.Exit(1)
	}

	apiClient := linkedin.NewAPIClient(cfg.LiAt, cfg.LiRm, cfg.JSessionID, cfg.CookieHeader)

	fmt.Printf("[INFO] Authenticating with LinkedIn API using session cookies...\n")
	fmt.Printf("[INFO] Fetching connections for profile: %s %s (%s)...\n", info.FirstName, info.LastName, info.URL)
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

	verifiedContacts := verifyConnectionsList(contacts, senderDomain, timeout, throttleDelay)

	excelPath, err := saveContactsToExcel(excelFileName, verifiedContacts)
	if err != nil {
		fmt.Printf("[ERROR] Failed to save verified contacts Excel workbook: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n==================================================")
	fmt.Printf("[SUCCESS] Finished! Verified results saved to Excel: %s\n", excelPath)
	fmt.Println("==================================================")
}

func handleLinkedInExport(exportPath, senderDomain string, timeout, throttleDelay time.Duration, autoProceed bool) {
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

	verifiedContacts := verifyConnectionsList(contacts, senderDomain, timeout, throttleDelay)

	excelPath, err := saveContactsToExcel(excelFileName, verifiedContacts)
	if err != nil {
		fmt.Printf("[ERROR] Failed to save verified contacts Excel workbook: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n==================================================")
	fmt.Printf("[SUCCESS] Finished! Verified results saved to Excel: %s\n", excelPath)
	fmt.Println("==================================================")
}

func verifyConnectionsList(contacts []models.Contact, senderDomain string, timeout, throttleDelay time.Duration) []models.Contact {
	total := len(contacts)
	for i, c := range contacts {
		fmt.Printf("\n[%d/%d] Processing Connection: %s %s\n", i+1, total, c.FirstName, c.LastName)

		targetDomain := c.Domain
		firstName := c.FirstName
		lastName := c.LastName

		if targetDomain == "" {
			if c.LinkedInURL != "" {
				fmt.Printf("[INFO] No domain specified. Scraping LinkedIn profile: %s\n", c.LinkedInURL)
				meta, err := linkedin.ScrapeProfile(c.LinkedInURL)
				if err == nil && meta.Company != "" {
					fmt.Printf("[INFO] Discovered company from profile: %s\n", meta.Company)
					resolvedDomain, errResolve := linkedin.ResolveCompanyDomain(meta.Company)
					if errResolve == nil && resolvedDomain != "" {
						targetDomain = resolvedDomain
						fmt.Printf("[INFO] Resolved corporate domain: %s\n", targetDomain)
					}
				}
			}
			if targetDomain == "" {
				targetDomain = "gmail.com"
				fmt.Println("[WARN] No corporate domain found. Defaulting to gmail.com.")
			}
		}

		contacts[i].Domain = targetDomain
		mxHost, err := smtp.ResolvePrimaryMX(targetDomain)
		if err != nil {
			fmt.Printf("[ERROR] DNS MX lookup failed for %s: %v\n", targetDomain, err)
			contacts[i].Status = "MX Lookup Failed"
			contacts[i].CampaignSent = "No"
			continue
		}
		fmt.Printf("[INFO] Primary MX resolved: %s\n", mxHost)

		if smtp.IsCatchAll(mxHost, targetDomain, senderDomain, timeout) {
			fmt.Printf("[WARN] Domain %s is Catch-All. Skipping live probing.\n", targetDomain)
			contacts[i].Status = "Catch-All Detected"
			contacts[i].CampaignSent = "No"
			continue
		}

		candidates := linkedin.GetCandidatePermutations(firstName, lastName, targetDomain)
		var verifiedEmail string

		fmt.Println("[INFO] Live Probing Permutations via SMTP (Port 25):")
		for _, candidate := range candidates {
			fmt.Printf("  -> Probing %s ... ", candidate)
			if smtp.ProbeMailbox(mxHost, candidate, senderDomain, timeout) {
				fmt.Println("[VALID]")
				verifiedEmail = candidate
				break
			}
			fmt.Println("[REJECTED]")
			time.Sleep(throttleDelay)
		}

		if verifiedEmail != "" {
			fmt.Printf("  [VALID] %s\n", verifiedEmail)
			contacts[i].VerifiedEmail = verifiedEmail
			contacts[i].Status = "Verified"
			contacts[i].CampaignSent = "Pending"
		} else {
			fmt.Println("  [INFO] No valid permutation detected.")
			contacts[i].Status = "Not Found"
			contacts[i].CampaignSent = "No"
		}
	}

	return contacts
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

func handleLinkedInLookup(rawURL, domain, senderDomain string, timeout, throttleDelay time.Duration) {
	info, err := linkedin.ParseProfileURL(rawURL)
	if err != nil {
		fmt.Printf("[ERROR] Error parsing LinkedIn URL: %v\n", err)
		os.Exit(1)
	}

	firstName := info.FirstName
	lastName := info.LastName

	fmt.Printf("[INFO] Parsed Profile URL: %s %s (Slug: %s)\n", firstName, lastName, info.Slug)

	initialDomain := domain
	if initialDomain == "" {
		initialDomain = "gmail.com"
		fmt.Printf("[INFO] No domain specified with -domain flag. Defaulting to '%s'.\n", initialDomain)
	}

	verifiedEmail := probeDomainPermutations(rawURL, firstName, lastName, initialDomain, senderDomain, timeout, throttleDelay)

	if verifiedEmail != "" {
		fmt.Println("\n==================================================")
		fmt.Printf(" [SUCCESS] Found & Verified Email Address: %s\n", verifiedEmail)
		fmt.Println("==================================================")
		return
	}

	fmt.Printf("\n[INFO] No verified email found on initial domain '%s'.\n", initialDomain)
	fmt.Println("[INFO] Scraping LinkedIn profile to discover active company & corporate domain...")

	meta, err := linkedin.ScrapeProfile(rawURL)
	if err != nil {
		fmt.Printf("[ERROR] Profile scraping failed: %v\n", err)
		return
	}

	activeCompany := meta.Company
	if activeCompany == "" {
		fmt.Println("[WARN] Could not identify active company from public profile.")
		return
	}

	fmt.Printf("[SUCCESS] Identified Active Company: %s\n", activeCompany)

	companyDomain := meta.Domain
	if companyDomain == "" {
		var resolveErr error
		companyDomain, resolveErr = linkedin.ResolveCompanyDomain(activeCompany)
		if resolveErr != nil || companyDomain == "" {
			fmt.Printf("[ERROR] Could not resolve corporate domain for '%s': %v\n", activeCompany, resolveErr)
			return
		}
	}

	fmt.Printf("[SUCCESS] Resolved Corporate Domain: %s\n", companyDomain)

	corpEmail := probeDomainPermutations(rawURL, firstName, lastName, companyDomain, senderDomain, timeout, throttleDelay)
	if corpEmail != "" {
		fmt.Println("\n==================================================")
		fmt.Printf(" [SUCCESS] Found & Verified Corporate Email Address: %s\n", corpEmail)
		fmt.Println("==================================================")
		return
	}

	fmt.Println("\n==================================================")
	fmt.Println(" [-] Finished. No verified email detected across domains.")
	fmt.Println("==================================================")
}

func probeDomainPermutations(rawURL, firstName, lastName, domain, senderDomain string, timeout, throttleDelay time.Duration) string {
	fmt.Printf("\n[*] Resolving MX servers for domain: %s\n", domain)
	mxHost, err := smtp.ResolvePrimaryMX(domain)
	if err != nil {
		fmt.Printf("[-] DNS MX resolution failed for '%s': %v\n", domain, err)
		return ""
	}
	fmt.Printf("[+] Primary MX Resolved: %s\n", mxHost)

	candidates := linkedin.GetCandidatePermutations(firstName, lastName, domain)
	fmt.Printf("[*] Generated %d permutation candidates for testing.\n", len(candidates))

	if smtp.IsCatchAll(mxHost, domain, senderDomain, timeout) {
		fmt.Printf("[WARN] Domain '%s' is Catch-All. Skipping live probing.\n", domain)
		return ""
	}

	fmt.Println("\n[INFO] Live Probing Permutations via SMTP (Port 25):")
	for _, candidate := range candidates {
		fmt.Printf("  -> Probing %s ... ", candidate)
		if smtp.ProbeMailbox(mxHost, candidate, senderDomain, timeout) {
			fmt.Println("[VALID]")
			return candidate
		}
		fmt.Println("[REJECTED]")
		time.Sleep(throttleDelay)
	}

	return ""
}

func handleExcelBatch(inputPath, outputPath, senderDomain string, timeout, throttleDelay time.Duration) {
	fmt.Printf("[INFO] Loading records from: %s\n", inputPath)

	var contacts []models.Contact
	var handler *excel.Handler
	isCSV := strings.HasSuffix(strings.ToLower(inputPath), ".csv")

	if isCSV {
		var err error
		contacts, err = csv.ReadContactsFromCSV(inputPath)
		if err != nil {
			log.Fatalf("[ERROR] Failed to open CSV input file: %v", err)
		}
	} else {
		var err error
		handler, contacts, err = excel.OpenSpreadsheet(inputPath)
		if err != nil {
			log.Fatalf("[ERROR] Failed to open input file: %v", err)
		}
		defer handler.Close()
	}

	total := len(contacts)
	fmt.Printf("[INFO] Loaded %d contact records.\n\n", total)

	for i := range contacts {
		c := &contacts[i]
		fmt.Printf("[%d/%d] Checking: %s %s (@%s)\n", i+1, total, c.FirstName, c.LastName, c.Domain)

		if c.FirstName == "" {
			fmt.Println("  [ERROR] Incomplete contact details (missing first name).")
			if handler != nil {
				_ = handler.UpdateRow(c.RowIndex, "", "Skipped (Missing Data)", "No")
			}
			c.Status = "Skipped"
			continue
		}

		if c.Domain == "" {
			if c.Status != "" {
				if comp := linkedin.ExtractCompanyFromHeadline(c.Status); comp != "" {
					if d, errD := linkedin.ResolveCompanyDomain(comp); errD == nil {
						c.Domain = d
					}
				}
			}
			if c.Domain == "" && c.LinkedInURL != "" {
				if meta, errS := linkedin.ScrapeProfile(c.LinkedInURL); errS == nil && meta != nil {
					if meta.Domain != "" {
						c.Domain = meta.Domain
					} else if meta.Company != "" {
						if d, errD := linkedin.ResolveCompanyDomain(meta.Company); errD == nil {
							c.Domain = d
						}
					}
				}
			}
			if c.Domain == "" {
				c.Domain = "gmail.com"
				fmt.Println("  [WARN] Defaulting domain to gmail.com")
			} else {
				fmt.Printf("  [INFO] Discovered company domain: %s\n", c.Domain)
			}
		}

		mxHost, err := smtp.ResolvePrimaryMX(c.Domain)
		if err != nil {
			fmt.Printf("  [ERROR] DNS MX lookup failed: %v\n", err)
			if handler != nil {
				_ = handler.UpdateRow(c.RowIndex, "", "MX Lookup Failed", "No")
			}
			c.Status = "MX Lookup Failed"
			continue
		}
		fmt.Printf("  [INFO] Primary MX resolved: %s\n", mxHost)

		if smtp.IsCatchAll(mxHost, c.Domain, senderDomain, timeout) {
			fmt.Printf("  [WARN] Domain '%s' is Catch-All. Skipping permutations.\n", c.Domain)
			if handler != nil {
				_ = handler.UpdateRow(c.RowIndex, "", "Catch-All Detected", "No")
			}
			c.Status = "Catch-All Detected"
			continue
		}

		candidates := linkedin.GetCandidatePermutations(c.FirstName, c.LastName, c.Domain)
		var verifiedEmail string

		for _, candidate := range candidates {
			fmt.Printf("    -> Probing: %s ... ", candidate)
			if smtp.ProbeMailbox(mxHost, candidate, senderDomain, timeout) {
				fmt.Println("[VALID]")
				verifiedEmail = candidate
				break
			}
			fmt.Println("[REJECTED/TIMEOUT]")
			time.Sleep(throttleDelay)
		}

		if verifiedEmail != "" {
			fmt.Printf("  [VALID] %s\n", verifiedEmail)
			c.VerifiedEmail = verifiedEmail
			c.Status = "Verified"
			c.CampaignSent = "Pending"
			if handler != nil {
				_ = handler.UpdateRow(c.RowIndex, verifiedEmail, "Verified", "Pending")
			}
		} else {
			fmt.Println("  [INFO] No valid permutation detected.")
			c.Status = "Not Found"
			c.CampaignSent = "No"
			if handler != nil {
				_ = handler.UpdateRow(c.RowIndex, "", "Not Found", "No")
			}
		}
	}

	if handler != nil {
		if err := handler.SaveAs(outputPath); err != nil {
			log.Fatalf("[ERROR] Failed to save output workbook: %v", err)
		}
	} else {
		if err := excel.CreateSpreadsheet(outputPath, contacts); err != nil {
			log.Fatalf("[ERROR] Failed to save output workbook: %v", err)
		}
	}

	fmt.Println("\n==================================================")
	fmt.Printf("[SUCCESS] Finished. Results saved to: %s\n", outputPath)
	fmt.Println("==================================================")
}
