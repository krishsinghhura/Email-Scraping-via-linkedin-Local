package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"email-verifier-cli/internal/config"
	"email-verifier-cli/internal/csv"
	"email-verifier-cli/internal/excel"
	"email-verifier-cli/internal/linkedin"
	"email-verifier-cli/internal/models"
	"email-verifier-cli/internal/smtp"
)

type TokenPayload struct {
	LiAt         string `json:"li_at"`
	LiRm         string `json:"li_rm"`
	JSessionID   string `json:"jsessionid"`
	CookieHeader string `json:"cookie_header"`
}

type ImportConnectionsPayload struct {
	ProfileSlug string           `json:"profile_slug"`
	Contacts    []models.Contact `json:"contacts"`
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func StartSetupServer(port int) error {
	fmt.Println("==================================================")
	fmt.Println("           LinkedIn Session Setup (Internal)      ")
	fmt.Println("==================================================")
	fmt.Println("[INFO] Scanning local browser profiles for LinkedIn session...")

	tokens, err := ExtractLinkedInTokensInternally()
	if err == nil && tokens != nil && tokens.LiAt != "" {
		cfg, _ := config.LoadConfig()
		cfg.LiAt = tokens.LiAt
		if tokens.LiRm != "" {
			cfg.LiRm = tokens.LiRm
		}
		if tokens.JSessionID != "" {
			cfg.JSessionID = tokens.JSessionID
		}
		if errSave := config.SaveConfig(cfg); errSave == nil {
			fmt.Printf("[SUCCESS] Automatically captured session credentials from browser!\n")
			fmt.Printf("[INFO] Saved configuration to: %s\n", config.GetConfigPath())
			fmt.Printf("[SUCCESS] Setup complete! You can now run:\n")
			fmt.Printf("  email-verifier -fetch-connections https://www.linkedin.com/in/your-profile-slug\n\n")
			return nil
		}
	}

	fmt.Println("[WARN] Could not automatically extract cookies from browser databases.")
	fmt.Print("Please paste your LinkedIn 'li_at' cookie manually: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = config.CleanCookieValue(line)
	if line != "" {
		cfg, _ := config.LoadConfig()
		cfg.LiAt = line
		_ = config.SaveConfig(cfg)
		fmt.Printf("\n[SUCCESS] Configuration saved to: %s\n\n", config.GetConfigPath())
		return nil
	}

	return fmt.Errorf("setup cancelled")
}

func StartTokenSyncServer(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/update-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload TokenPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(Response{Success: false, Message: "Invalid JSON body"})
			return
		}

		cfg, _ := config.LoadConfig()
		if payload.LiAt != "" {
			cfg.LiAt = config.CleanCookieValue(payload.LiAt)
		}
		if payload.LiRm != "" {
			cfg.LiRm = config.CleanCookieValue(payload.LiRm)
		}
		if payload.JSessionID != "" {
			cfg.JSessionID = config.CleanCookieValue(payload.JSessionID)
		}
		if payload.CookieHeader != "" {
			cfg.CookieHeader = strings.TrimSpace(payload.CookieHeader)
		}

		_ = config.SaveConfig(cfg)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response{Success: true, Message: "Token updated successfully"})
	})

	mux.HandleFunc("/import-connections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload ImportConnectionsPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(Response{Success: false, Message: "Invalid JSON body"})
			return
		}

		slug := payload.ProfileSlug
		if slug == "" {
			slug = "connections_export"
		}

		csvFile := fmt.Sprintf("%s_connections.csv", slug)
		csvPath, err := csv.SaveConnectionsToCSV(csvFile, payload.Contacts)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(Response{Success: false, Message: err.Error()})
			return
		}

		go processImportedContacts(slug, payload.Contacts)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response{
			Success: true,
			Message: fmt.Sprintf("Imported %d contacts. CSV saved to %s.", len(payload.Contacts), csvPath),
		})
	})

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return srv.ListenAndServe()
}

func processImportedContacts(slug string, contacts []models.Contact) {
	senderDomain := "example.com"
	timeout := 5 * time.Second
	throttleDelay := 250 * time.Millisecond

	total := len(contacts)
	for i, c := range contacts {
		fmt.Printf("[%d/%d] Checking: %s %s (@%s)\n", i+1, total, c.FirstName, c.LastName, c.Domain)

		targetDomain := c.Domain
		if targetDomain == "" {
			if comp := extractCompanyFromHeadline(c.Status); comp != "" {
				if dom, err := linkedin.ResolveCompanyDomain(comp); err == nil {
					targetDomain = dom
					contacts[i].Domain = dom
				}
			}
		}

		if targetDomain == "" {
			contacts[i].Status = "Skipped (No Domain)"
			continue
		}

		mxHost, err := smtp.ResolvePrimaryMX(targetDomain)
		if err != nil {
			contacts[i].Status = "MX Lookup Failed"
			continue
		}

		if smtp.IsCatchAll(mxHost, targetDomain, senderDomain, timeout) {
			contacts[i].Status = "Catch-All Detected"
			continue
		}

		candidates := linkedin.GetCandidatePermutations(c.FirstName, c.LastName, targetDomain)
		var verifiedEmail string
		for _, cand := range candidates {
			if smtp.ProbeMailbox(mxHost, cand, senderDomain, timeout) {
				verifiedEmail = cand
				break
			}
			time.Sleep(throttleDelay)
		}

		if verifiedEmail != "" {
			fmt.Printf("  [VALID] %s\n", verifiedEmail)
			contacts[i].VerifiedEmail = verifiedEmail
			contacts[i].Status = "Verified"
			contacts[i].CampaignSent = "Pending"
		} else {
			contacts[i].Status = "Not Found"
			contacts[i].CampaignSent = "No"
		}
	}

	excelFile := fmt.Sprintf("%s_verified_contacts.xlsx", slug)
	_ = excel.CreateSpreadsheet(excelFile, contacts)
	fmt.Printf("\n[SUCCESS] Verified Excel saved to: %s\n\n", excelFile)
}

func extractCompanyFromHeadline(headline string) string {
	if headline == "" {
		return ""
	}
	parts := strings.Split(headline, "@")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	parts = strings.Split(headline, " at ")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}
