package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Store struct {
	filePath       string
	mu             sync.RWMutex
	CompanyDomains map[string]string `json:"company_domains"`
	DomainPatterns map[string]string `json:"domain_patterns"`
}

var (
	globalStore *Store
	once        sync.Once
)

func GetGlobalStore() *Store {
	once.Do(func() {
		home, err := os.UserHomeDir()
		cachePath := ".email-verifier-cache.json"
		if err == nil {
			cachePath = filepath.Join(home, cachePath)
		}
		globalStore = NewStore(cachePath)
	})
	return globalStore
}

func NewStore(filePath string) *Store {
	s := &Store{
		filePath:       filePath,
		CompanyDomains: make(map[string]string),
		DomainPatterns: make(map[string]string),
	}
	s.Load()
	return s
}

func (s *Store) Load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	var loaded struct {
		CompanyDomains map[string]string `json:"company_domains"`
		DomainPatterns map[string]string `json:"domain_patterns"`
	}

	if err := json.Unmarshal(data, &loaded); err == nil {
		if loaded.CompanyDomains != nil {
			s.CompanyDomains = loaded.CompanyDomains
		}
		if loaded.DomainPatterns != nil {
			s.DomainPatterns = loaded.DomainPatterns
		}
	}
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Store) GetCompanyDomain(companyName string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := normalizeCompanyKey(companyName)
	dom, ok := s.CompanyDomains[key]
	return dom, ok
}

func (s *Store) SetCompanyDomain(companyName, domain string) {
	key := normalizeCompanyKey(companyName)
	domain = strings.ToLower(strings.TrimSpace(domain))
	if key == "" || domain == "" {
		return
	}

	s.mu.Lock()
	s.CompanyDomains[key] = domain
	s.mu.Unlock()

	_ = s.Save()
}

func (s *Store) GetDomainPattern(domain string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	domain = strings.ToLower(strings.TrimSpace(domain))
	pat, ok := s.DomainPatterns[domain]
	return pat, ok
}

func (s *Store) SetDomainPattern(domain, pattern string) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	pattern = strings.TrimSpace(pattern)
	if domain == "" || pattern == "" {
		return
	}

	s.mu.Lock()
	s.DomainPatterns[domain] = pattern
	s.mu.Unlock()

	_ = s.Save()
}

func normalizeCompanyKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}
