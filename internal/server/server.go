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

func CopyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			return nil
		}
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func GetAutoSyncScript(port int) string {
	return fmt.Sprintf(`(async function(){function g(n){let m=document.cookie.match(new RegExp('(^|; )'+n+'=([^;]+)'));return m?decodeURIComponent(m[2]):'';}try{let r=await fetch('http://127.0.0.1:%d/update-token',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({li_at:g('li_at'),li_rm:g('li_rm'),jsessionid:g('JSESSIONID'),cookie_header:document.cookie})});let d=await r.json();if(d.success){console.log('%%c[SUCCESS] LinkedIn session synced! Return to your terminal.','color:#22c55e;font-size:16px;font-weight:bold;');alert('[SUCCESS] LinkedIn session synced! You can return to your terminal.');}else{alert('[ERROR] '+(d.message||'Failed to save session.'));}}catch(e){alert('[ERROR] Could not connect to setup server on port %d.');}})();`, port, port)
}

func StartSetupServer(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	syncScript := GetAutoSyncScript(port)

	_ = CopyToClipboard(syncScript)

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

		fmt.Printf("\n[SUCCESS] Received session credentials from LinkedIn!\n")
		fmt.Printf("[INFO] Saved configuration to: %s\n", config.GetConfigPath())
		fmt.Printf("[SUCCESS] Setup complete! You can now run:\n")
		fmt.Printf("  email-verifier -fetch-connections https://www.linkedin.com/in/your-profile-slug\n\n")

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

	fmt.Println("==================================================")
	fmt.Println("           Automated LinkedIn Session Setup       ")
	fmt.Println("==================================================")
	fmt.Println("[INFO] Opening https://www.linkedin.com in your default browser...")
	fmt.Println("[INFO] Auto-sync script has been copied to your clipboard.")
	fmt.Println("\n--------------------------------------------------")
	fmt.Println("Quick 1-step sync on the opened LinkedIn tab:")
	if runtime.GOOS == "darwin" {
		fmt.Println("1. Press Cmd + Option + J (opens Developer Console)")
		fmt.Println("2. Press Cmd + V (Paste) and hit Enter")
	} else {
		fmt.Println("1. Press Ctrl + Shift + J (opens Developer Console)")
		fmt.Println("2. Press Ctrl + V (Paste) and hit Enter")
	}
	fmt.Println("--------------------------------------------------")
	fmt.Printf("[INFO] Waiting for session sync from browser (listening on %s)...\n", addr)

	OpenBrowser("https://www.linkedin.com/feed/")

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
  <title>LinkedIn Session Setup</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
    body { background: #0f172a; color: #f8fafc; display: flex; justify-content: center; align-items: center; min-height: 100vh; padding: 24px; }
    .card { background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: 36px; max-width: 600px; width: 100%; box-shadow: 0 10px 25px rgba(0,0,0,0.5); }
    h1 { font-size: 22px; margin-bottom: 8px; font-weight: 600; color: #38bdf8; }
    p { font-size: 14px; color: #94a3b8; margin-bottom: 24px; line-height: 1.5; }
    .box { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 16px; margin-bottom: 20px; }
    .box h3 { font-size: 14px; color: #e2e8f0; margin-bottom: 8px; }
    .box p { margin-bottom: 12px; font-size: 13px; }
    .btn { display: inline-block; width: 100%; text-align: center; background: #0284c7; color: white; border: none; padding: 12px; font-size: 15px; font-weight: 600; border-radius: 6px; cursor: pointer; text-decoration: none; }
    .btn:hover { background: #0369a1; }
  </style>
</head>
<body>
  <div class="card">
    <h1>LinkedIn Session Setup</h1>
    <p>Automated authentication sync for email-verifier. The script was copied to your clipboard when you ran the setup command.</p>

    <div class="box">
      <h3>Automatic Setup:</h3>
      <p>1. Open LinkedIn in your browser.<br>2. Press <b>Cmd + Option + J</b> (or Ctrl + Shift + J).<br>3. Press <b>Cmd + V</b> (Paste) and hit <b>Enter</b>.</p>
      <a href="https://www.linkedin.com/feed/" target="_blank" class="btn">Open LinkedIn Now</a>
    </div>
  </div>
</body>
</html>
`
