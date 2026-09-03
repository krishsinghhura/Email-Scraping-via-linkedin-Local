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

### 4. Automated LinkedIn Session Setup (Only for Exporting Own Connections)

If you want to fetch and export your own LinkedIn connections directly via the CLI:

```bash
email-verifier setup
```

1. The CLI automatically opens LinkedIn in your default browser.
2. The auto-sync script is automatically copied to your clipboard.
3. On the opened LinkedIn tab, open Developer Console (`Cmd+Option+J` on macOS or `Ctrl+Shift+J` on Windows/Linux).
4. Press `Cmd+V` (Paste) and hit **Enter**.
5. The session tokens are automatically received by the CLI and saved to `~/.email-verifier-config.json`.

After setup, you can immediately fetch your connection list:

```bash
email-verifier -fetch-connections https://www.linkedin.com/in/your-profile-slug/
```

---

## CLI Flags Reference

| Flag | Default | Description |
|---|---|---|
| `setup` | `false` | Launch the browser-based configuration setup on `http://localhost:9876/setup` |
| `-url` | `""` | Target LinkedIn profile URL for single-contact lookup |
| `-domain` | `""` | Target corporate domain (used with `-url`) |
| `-input` | `contacts.xlsx` | Path to input Excel (`.xlsx`), CSV (`.csv`), or ZIP export archive |
| `-output` | `verified_campaign.xlsx` | Path to output Excel file |
| `-import-export` | `""` | Path to LinkedIn data export `.zip` or `Connections.csv` |
| `-fetch-connections` | `""` | Profile URL to fetch 1st-degree connections from |
| `-limit` | `100` | Maximum number of connections to fetch |
| `-sender-domain` | `example.com` | Domain used in SMTP HELO and MAIL FROM commands |
| `-timeout` | `5` | Connection and socket read timeout in seconds |
| `-delay` | `250` | Throttle delay in milliseconds between mailbox probes |
| `-y`, `-yes` | `false` | Auto-confirm prompt to proceed with verification |
| `-refresh-session` | `false` | Force refresh session `li_at` using stored `li_rm` |

---

## How Email Verification Works

1. **Permutation Generation**: The tool generates 15 standard corporate patterns (e.g., `first.last@domain.com`, `f.last@domain.com`, `first@domain.com`, `lastf@domain.com`).
2. **DNS MX Resolution**: Identifies the primary mail exchangers for the target domain.
3. **Catch-All Detection**: Sends a probe for a non-existent randomized address. If accepted, the domain is marked as Catch-All and individual addresses cannot be verified via SMTP.
4. **Zero-Deliverability Probe**: Initiates an SMTP connection on port 25, sends `HELO`, `MAIL FROM`, and `RCPT TO:<candidate>`, inspects the 250 OK or 550 User Not Found response, and issues `RSET`/`QUIT` without transmitting any email content.

---

## License

MIT License.
