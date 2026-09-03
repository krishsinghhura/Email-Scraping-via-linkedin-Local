package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

type ExtractedTokens struct {
	LiAt       string
	LiRm       string
	JSessionID string
}

func DecryptChromiumCookie(encryptedHex, password string) (string, error) {
	encBytes, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", err
	}

	if len(encBytes) < 3 {
		return "", fmt.Errorf("encrypted cookie too short")
	}

	if string(encBytes[:3]) != "v10" {
		return string(encBytes), nil
	}

	ciphertext := encBytes[3:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of block size")
	}

	salt := []byte("saltysalt")
	iv := bytes.Repeat([]byte(" "), 16)
	key := pbkdf2.Key([]byte(password), salt, 1003, 16, sha1.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	if len(decrypted) == 0 {
		return "", fmt.Errorf("empty decrypted payload")
	}

	padLen := int(decrypted[len(decrypted)-1])
	if padLen > 0 && padLen <= aes.BlockSize {
		decrypted = decrypted[:len(decrypted)-padLen]
	}

	return string(decrypted), nil
}

func GetKeychainPassword(serviceName, accountName string) (string, error) {
	args := []string{"find-generic-password", "-w", "-s", serviceName}
	if accountName != "" {
		args = append(args, "-a", accountName)
	}
	out, err := exec.Command("security", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func ExtractLinkedInTokensInternally() (*ExtractedTokens, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("automated internal extraction is currently supported on macOS")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	type browserTarget struct {
		name        string
		cookieGlobs []string
		service     string
		account     string
	}

	targets := []browserTarget{
		{
			name: "Brave Browser",
			cookieGlobs: []string{
				filepath.Join(home, "Library/Application Support/BraveSoftware/Brave-Browser/Default/Cookies"),
				filepath.Join(home, "Library/Application Support/BraveSoftware/Brave-Browser/Profile */Cookies"),
			},
			service: "Brave Safe Storage",
			account: "Brave",
		},
		{
			name: "Google Chrome",
			cookieGlobs: []string{
				filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Cookies"),
				filepath.Join(home, "Library/Application Support/Google/Chrome/Profile */Cookies"),
			},
			service: "Chrome Safe Storage",
			account: "Chrome",
		},
		{
			name: "Arc",
			cookieGlobs: []string{
				filepath.Join(home, "Library/Application Support/Arc/User Data/Default/Cookies"),
				filepath.Join(home, "Library/Application Support/Arc/User Data/Profile */Cookies"),
			},
			service: "Arc Safe Storage",
			account: "Arc",
		},
		{
			name: "Microsoft Edge",
			cookieGlobs: []string{
				filepath.Join(home, "Library/Application Support/Microsoft Edge/Default/Cookies"),
				filepath.Join(home, "Library/Application Support/Microsoft Edge/Profile */Cookies"),
			},
			service: "Microsoft Edge Safe Storage",
			account: "Microsoft Edge",
		},
	}

	for _, target := range targets {
		var matchedFiles []string
		for _, g := range target.cookieGlobs {
			m, _ := filepath.Glob(g)
			matchedFiles = append(matchedFiles, m...)
		}

		for _, cookieFile := range matchedFiles {
			if _, statErr := os.Stat(cookieFile); statErr != nil {
				continue
			}

			cmdStr := fmt.Sprintf("file:%s?mode=ro", cookieFile)
			query := "SELECT name, hex(encrypted_value) FROM cookies WHERE host_key LIKE '%linkedin.com%' AND name IN ('li_at', 'JSESSIONID', 'li_rm')"
			out, errQ := exec.Command("sqlite3", cmdStr, query).Output()
			if errQ != nil || len(out) == 0 {
				continue
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			rawTokens := make(map[string]string)
			for _, line := range lines {
				parts := strings.Split(line, "|")
				if len(parts) == 2 {
					rawTokens[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}

			if rawTokens["li_at"] == "" {
				continue
			}

			password, pwErr := GetKeychainPassword(target.service, target.account)
			if pwErr != nil {
				password, pwErr = GetKeychainPassword(target.service, "")
			}
			if pwErr != nil || password == "" {
				continue
			}

			liAt, errLiAt := DecryptChromiumCookie(rawTokens["li_at"], password)
			if errLiAt != nil || liAt == "" {
				continue
			}

			tokens := &ExtractedTokens{
				LiAt: liAt,
			}

			if rawTokens["JSESSIONID"] != "" {
				if jsession, errJ := DecryptChromiumCookie(rawTokens["JSESSIONID"], password); errJ == nil {
					tokens.JSessionID = jsession
				}
			}
			if rawTokens["li_rm"] != "" {
				if liRm, errRm := DecryptChromiumCookie(rawTokens["li_rm"], password); errRm == nil {
					tokens.LiRm = liRm
				}
			}

			return tokens, nil
		}
	}

	return nil, fmt.Errorf("no active LinkedIn session found in local browser cookie databases")
}
