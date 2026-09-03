package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
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

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func StartSetupServer(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	setupURL := fmt.Sprintf("http://localhost:%d/setup", port)

	mux := http.NewServeMux()

	var srv *http.Server
	srv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, setupHTML)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

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

		if err := config.SaveConfig(cfg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(Response{Success: false, Message: "Failed to save configuration"})
			return
		}

		fmt.Printf("\n[INFO] Configuration saved to: %s\n", config.GetConfigPath())
		fmt.Printf("[SUCCESS] Setup complete. You can now return to the terminal.\n\n")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response{Success: true, Message: "Configuration saved successfully. Setup complete."})

		go func() {
			time.Sleep(1 * time.Second)
			_ = srv.Shutdown(context.Background())
		}()
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

		if len(payload.Contacts) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(Response{Success: false, Message: "No contacts provided"})
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

		fmt.Printf("\n[INFO] Received %d connections from browser.\n", len(payload.Contacts))
		fmt.Printf("[INFO] Saved to CSV: %s\n", csvPath)
		fmt.Printf("[INFO] Running background email verification...\n")

		go processImportedContacts(slug, payload.Contacts)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response{
			Success: true,
			Message: fmt.Sprintf("Imported %d contacts. CSV saved to %s.", len(payload.Contacts), csvPath),
		})
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		cfg, err := config.LoadConfig()
		if err != nil || (cfg.LiAt == "" && cfg.CookieHeader == "") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"active":      true,
			"has_li_at":   len(cfg.LiAt) > 0,
			"config_path": config.GetConfigPath(),
		})
	})

	fmt.Printf("[INFO] Starting Setup Server at %s\n", setupURL)
	fmt.Printf("[INFO] Opening default browser...\n")
	OpenBrowser(setupURL)

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
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

const setupHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Email Verifier - LinkedIn Setup</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    body { background: #0f172a; color: #f8fafc; display: flex; justify-content: center; align-items: center; min-height: 100vh; padding: 24px; }
    .card { background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: 36px; max-width: 600px; width: 100%; box-shadow: 0 10px 25px rgba(0,0,0,0.5); }
    h1 { font-size: 22px; margin-bottom: 8px; font-weight: 600; color: #38bdf8; }
    p { font-size: 14px; color: #94a3b8; margin-bottom: 24px; line-height: 1.5; }
    .field { margin-bottom: 18px; }
    label { display: block; font-size: 13px; font-weight: 500; margin-bottom: 6px; color: #cbd5e1; }
    input[type="text"], textarea { width: 100%; background: #0f172a; border: 1px solid #475569; border-radius: 6px; padding: 10px 12px; font-size: 14px; color: #f1f5f9; outline: none; }
    input[type="text"]:focus, textarea:focus { border-color: #38bdf8; }
    textarea { height: 75px; resize: vertical; font-family: monospace; font-size: 12px; }
    button { width: 100%; background: #0284c7; color: white; border: none; padding: 12px; font-size: 15px; font-weight: 600; border-radius: 6px; cursor: pointer; transition: background 0.2s; margin-top: 8px; }
    button:hover { background: #0369a1; }
    .snippet-box { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 16px; margin-bottom: 24px; }
    .snippet-title { font-size: 13px; font-weight: 600; margin-bottom: 8px; color: #f8fafc; }
    .status { margin-top: 16px; font-size: 14px; text-align: center; display: none; }
    .status.success { color: #4ade80; display: block; }
    .status.error { color: #f87171; display: block; }
  </style>
</head>
<body>
  <div class="card">
    <h1>LinkedIn Connection Setup</h1>
    <p>Configure your LinkedIn session cookies to enable direct connection exports and automated verification. This setup is only needed if you want to export your own connections.</p>
    
    <div class="field">
      <label for="li_at">li_at Cookie Token</label>
      <input type="text" id="li_at" placeholder="AQED...">
    </div>

    <div class="field">
      <label for="li_rm">li_rm Persistent Refresh Token (Optional)</label>
      <input type="text" id="li_rm" placeholder="AQFD...">
    </div>

    <div class="field">
      <label for="jsessionid">JSESSIONID Cookie Token (Optional)</label>
      <input type="text" id="jsessionid" placeholder="ajax:...">
    </div>

    <div class="field">
      <label for="cookie_header">Or Paste Full Cookie String</label>
      <textarea id="cookie_header" placeholder="bcookie=...; li_at=...; JSESSIONID=..."></textarea>
    </div>

    <button id="saveBtn">Save Configuration</button>
    <div id="status" class="status"></div>
  </div>

  <script>
    document.getElementById("saveBtn").addEventListener("click", async () => {
      const liAt = document.getElementById("li_at").value.trim();
      const liRm = document.getElementById("li_rm").value.trim();
      const jsessionId = document.getElementById("jsessionid").value.trim();
      const cookieHeader = document.getElementById("cookie_header").value.trim();
      const statusEl = document.getElementById("status");

      if (!liAt && !cookieHeader) {
        statusEl.className = "status error";
        statusEl.innerText = "Please provide at least li_at or the full Cookie string.";
        return;
      }

      statusEl.className = "status";
      statusEl.innerText = "Saving configuration...";
      statusEl.style.display = "block";

      try {
        const resp = await fetch("/update-token", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            li_at: liAt,
            li_rm: liRm,
            jsessionid: jsessionId,
            cookie_header: cookieHeader
          })
        });
        const data = await resp.json();
        if (data.success) {
          statusEl.className = "status success";
          statusEl.innerText = "[SUCCESS] Configuration saved. Setup complete! You can return to your terminal.";
        } else {
          statusEl.className = "status error";
          statusEl.innerText = "[ERROR] " + (data.message || "Failed to save.");
        }
      } catch (err) {
        statusEl.className = "status error";
        statusEl.innerText = "[ERROR] Could not connect to local setup server.";
      }
    });
  </script>
</body>
</html>
`
