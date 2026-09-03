package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	LiAt         string `json:"li_at"`
	LiRm         string `json:"li_rm"`
	JSessionID   string `json:"jsessionid"`
	CookieHeader string `json:"cookie_header,omitempty"`
}

func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".email-verifier-config.json"
	}
	return filepath.Join(home, ".email-verifier-config.json")
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func CleanCookieValue(val string) string {
	val = strings.TrimSpace(val)
	val = strings.Trim(val, "\"")
	val = strings.Trim(val, "'")
	return val
}
