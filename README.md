# Email Permutation and Verifier CLI

A high-performance command-line tool and email permutation engine built in Go. It discovers, generates, and validates corporate email addresses using direct LinkedIn profile scraping, official LinkedIn data exports, and real-time SMTP mailbox verification (RFC 5321) without sending any emails.

---

## Key Features

- **Direct Profile Email Discovery**: Resolves corporate domains and verifies email addresses for any public LinkedIn profile URL.
- **LinkedIn Connection Export & Verification**: Directly imports and verifies your LinkedIn connections.
- **Browser-Based Interactive Setup (`email-verifier setup`)**: An optional local setup screen to save your LinkedIn session cookies when you want to export your own connections.
- **LinkedIn Data Export Support**: Accepts `.zip` data export archives or `Connections.csv` files, extracting records and verifying corporate mailboxes.
- **Batch Excel & CSV Processing**: Reads spreadsheets (`.xlsx`) or `.csv` files, generates permutations, and saves results to an output workbook.
- **RFC 5321 SMTP Handshake Verification**: Connects directly to primary MX mail servers on port 25, checks for catch-all servers, and confirms mailbox deliverability.

---

## Installation

### Prerequisites

- Go 1.21 or newer installed.
- Network access to DNS and outbound TCP port 25 for SMTP mailbox probing.

### Install Globally

#### Option 1: 1-Line Quick Install (via curl)

```bash
curl -fsSL https://raw.githubusercontent.com/krishsinghhura/Email-Scraping-via-linkedin-Local/main/install.sh | bash
```

#### Option 2: Build From Source

Clone this repository and run:

```bash
make install
```

Or run the local installer:

```bash
chmod +x ./install.sh
./install.sh
```

This compiles and installs the binary to both `$(go env GOPATH)/bin` and `/opt/homebrew/bin` (or `/usr/local/bin`), making `email-verifier` globally accessible from any terminal window.

Verify installation:

```bash
email-verifier -help
```

---

## Usage Guide

### 1. Single Profile Email Lookup (No Setup Required)

Find the verified corporate email address of any individual by providing their public LinkedIn URL:

```bash
email-verifier -url https://www.linkedin.com/in/john-doe
```

If you already know the corporate domain, specify it explicitly to skip domain resolution:

```bash
email-verifier -url https://www.linkedin.com/in/john-doe -domain stripe.com
```

---

### 2. Batch Verification from Excel or CSV (No Setup Required)

Process an existing spreadsheet or CSV file of contacts:

```bash
email-verifier -input contacts.xlsx -output verified_results.xlsx
```

Or process a CSV file:

```bash
email-verifier -input contacts.csv -output verified_results.xlsx
```

The tool checks each contact's primary MX record, detects catch-all domains, and probes email permutations.

---

### 3. LinkedIn Data Export Ingestion (No Setup Required)

If you downloaded your data archive from LinkedIn:

```bash
email-verifier -import-export ~/Downloads/Connections.csv
```

Or pass the downloaded `.zip` archive directly:

```bash
email-verifier -import-export ~/Downloads/LinkedInDataExport.zip
```

The CLI will:
1. Extract all connection records.
2. Save a cleaned `{name}_connections.csv`.
3. Interactively prompt:
   ```text
   Do you want to proceed with finding email addresses for all X connections? (y/N):
   ```
4. If confirmed, generate permutations, probe mailboxes, and save results to `{name}_verified_contacts.xlsx`.

To bypass the prompt and proceed immediately, append `-y`:

```bash
email-verifier -import-export ~/Downloads/Connections.csv -y
```

---

### 4. Internal LinkedIn Session Setup (Only for Exporting Own Connections)

If you want to fetch and export your own LinkedIn connections directly via the CLI:

```bash
email-verifier setup
```

The CLI runs **100% internally within your terminal** without opening any browser or developer tools:
1. Automatically scans your local browser profiles (Brave, Chrome, Arc, Edge).
2. Reads and decrypts your active LinkedIn session cookies (`li_at`, `JSESSIONID`, `li_rm`).
3. If macOS prompts for permission to access your browser safe storage in Keychain, click **Always Allow**.
4. Automatically saves credentials to `~/.email-verifier-config.json`.

Once complete, you can immediately fetch and verify your connection list:

```bash
email-verifier -fetch-connections https://www.linkedin.com/in/your-profile-slug/
```

---

### 5. Prospecting & Searching Leads by Keyword or Title

Search LinkedIn for prospective leads (by job title, skills, or company) and verify their email addresses:

```bash
email-verifier -search "VP of Engineering San Francisco" -limit 25 -y
```

This will:
1. Search LinkedIn member profiles matching your query.
2. Auto-resolve the employer's domain.
3. Save discovered leads to `{query}_leads.csv`.
4. Concurrently verify permutations and save to `{query}_verified_leads.xlsx`.

---

## CLI Flags Reference

| Flag | Default | Description |
|---|---|---|
| `setup` | `false` | Launch the browser-based configuration setup on `http://localhost:9876/setup` |
| `-search` | `""` | Search LinkedIn for prospective leads by keyword or title |
| `-url` | `""` | Target LinkedIn profile URL for single-contact lookup |
| `-domain` | `""` | Target corporate domain (used with `-url`) |
| `-input` | `contacts.xlsx` | Path to input Excel (`.xlsx`), CSV (`.csv`), or ZIP export archive |
| `-output` | `verified_campaign.xlsx` | Path to output Excel file |
| `-import-export` | `""` | Path to LinkedIn data export `.zip` or `Connections.csv` |
| `-fetch-connections` | `""` | Profile URL to fetch 1st-degree connections from |
| `-limit` | `100` | Maximum number of connections or leads to fetch |
| `-sender-domain` | `example.com` | Domain used in SMTP HELO and MAIL FROM commands |
| `-timeout` | `5` | Connection and socket read timeout in seconds |
| `-delay` | `250` | Throttle delay in milliseconds between mailbox probes |
| `-concurrency` | `5` | Number of concurrent workers for email verification |
| `-y`, `-yes` | `false` | Auto-confirm prompt to proceed with verification |
| `-refresh-session` | `false` | Force refresh session `li_at` using stored `li_rm` |

---

## How Email Verification Works

1. **Permutation Generation**: The tool generates standard corporate email patterns (e.g., `first.last@domain.com`, `f.last@domain.com`, `first@domain.com`, `lastf@domain.com`).
2. **DNS MX Resolution**: Queries authoritative DNS records to discover primary mail exchangers handling inbound traffic for the domain.
3. **Catch-All Detection**: Runs a pre-flight probe using a cryptographically randomized, non-existent mailbox.
4. **Zero-Deliverability SMTP Handshake**: Performs an RFC 5321 handshake on TCP port 25, sends `HELO`, `MAIL FROM`, and `RCPT TO:<candidate>`, inspects response status codes (e.g., `250 OK` vs `550 User Not Found`), and issues `RSET`/`QUIT` without transmitting message body data.

---

## Understanding Catch-All (Accept-All) Domains

A **Catch-All domain** is configured on the mail server to accept all incoming emails, regardless of whether the recipient mailbox actually exists. For example, if a server is set to catch-all, sending an SMTP probe to `random-fake-user-12345@domain.com` will still return `250 OK`.

### Why Catch-All Detection Matters
If a tool does not detect catch-all configurations, every single generated permutation will falsely appear as valid, leading to low deliverability and bounced campaigns.

### How email-verifier Handles Catch-Alls
Before checking individual contact permutations:
1. The verifier sends a probe to a randomized mailbox like `catchall_chk_<hex>@domain.com`.
2. If the server responds with `250 OK`, the domain is classified as **Catch-All Detected**.
3. The tool flags the domain and skips further permutation probing to prevent false positive records and protect sender reputation.

---

## Outbound Port 25 Requirements & Troubleshooting

Email verification relies on standard Mail Transfer Agent (MTA) communication, which occurs over **TCP Port 25**. Many residential ISPs and cloud hosting providers block outbound traffic on port 25 by default to prevent unauthorized bulk spam.

### How to Test if Port 25 is Open

Run this check in your terminal or command prompt:

**Linux / macOS:**
```bash
nc -zv -w5 aspmx.l.google.com 25
```
Or:
```bash
telnet aspmx.l.google.com 25
```

**Windows (PowerShell):**
```powershell
Test-NetConnection -ComputerName aspmx.l.google.com -Port 25
```

If the connection succeeds, outbound port 25 is open. If it times out or is refused, follow the platform instructions below.

---

### How to Open Outbound Port 25

#### 1. Linux (Ubuntu, Debian, CentOS, RHEL)

If blocked by local firewall:

- **UFW (Ubuntu / Debian):**
  ```bash
  sudo ufw allow out 25/tcp
  sudo ufw reload
  ```

- **Firewalld (CentOS / RHEL / Fedora):**
  ```bash
  sudo firewall-cmd --permanent --direct --add-rule ipv4 filter OUTPUT 0 -p tcp -m tcp --dport 25 -j ACCEPT
  sudo firewall-cmd --reload
  ```

- **Iptables:**
  ```bash
  sudo iptables -A OUTPUT -p tcp --dport 25 -j ACCEPT
  ```

If running on a cloud VPS (e.g., AWS EC2, DigitalOcean, GCP, Hetzner, Vultr):
- Cloud providers block outbound port 25 on new accounts by default.
- Submit a ticket or request form in your cloud console (e.g., AWS "Request to remove email sending limitations" or DigitalOcean support ticket) requesting outbound port 25 unblocking.

---

#### 2. Windows

If blocked by Windows Defender Firewall:

- **PowerShell (Run as Administrator):**
  ```powershell
  New-NetFirewallRule -DisplayName "Allow Outbound SMTP Port 25" -Direction Outbound -LocalPort Any -Protocol TCP -RemotePort 25 -Action Allow
  ```

- **GUI Method:**
  1. Open **Windows Defender Firewall with Advanced Security**.
  2. Click **Outbound Rules** in the left sidebar.
  3. Click **New Rule...** in the right panel.
  4. Select **Port** and click Next.
  5. Choose **TCP** and specify port `25`.
  6. Select **Allow the connection** and click Next through to completion.

If your residential ISP blocks port 25 on your router:
- Check your router's administrative firewall settings and ensure outbound port 25 is not filtered.
- Alternatively, connect through a trusted VPN provider (like Mullvad, ProtonVPN, or a business VPN) or run the tool from a server/VPS that permits outbound port 25 traffic.

---

#### 3. macOS

- **Built-in Firewall:**
  Check **System Settings > Network > Firewall** to verify outbound connections are allowed, or disable the packet filter temporarily to test:
  ```bash
  sudo pfctl -d
  ```

- **ISP Restrictions:**
  Most home Wi-Fi networks block outbound port 25. If port 25 is blocked by your ISP, you can run the verifier while connected to a VPN service that allows outbound port 25, or run it from a remote development server.

---

## License

MIT License.
