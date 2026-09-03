package permutator

import (
	"fmt"
	"strings"
)

func GeneratePermutations(firstName, lastName, domain string) []string {
	fn := strings.ToLower(strings.TrimSpace(firstName))
	ln := strings.ToLower(strings.TrimSpace(lastName))
	dom := strings.ToLower(strings.TrimSpace(domain))

	if fn == "" || dom == "" {
		return nil
	}

	fInit := string(fn[0])
	lInit := ""
	if ln != "" {
		lInit = string(ln[0])
	}

	var rawPatterns []string

	if ln == "" {
		rawPatterns = []string{
			fmt.Sprintf("%s@%s", fn, dom),
		}
	} else {
		rawPatterns = []string{
			fmt.Sprintf("%s.%s@%s", fn, ln, dom),
			fmt.Sprintf("%s@%s", fn, dom),
			fmt.Sprintf("%s%s@%s", fn, ln, dom),
			fmt.Sprintf("%s%s@%s", fInit, ln, dom),
			fmt.Sprintf("%s_%s@%s", fn, ln, dom),
			fmt.Sprintf("%s@%s", ln, dom),
			fmt.Sprintf("%s.%s@%s", ln, fn, dom),
			fmt.Sprintf("%s.%s@%s", fInit, ln, dom),
			fmt.Sprintf("%s%s@%s", ln, fInit, dom),
			fmt.Sprintf("%s%s@%s", fn, lInit, dom),
			fmt.Sprintf("%s.%s@%s", fn, lInit, dom),
			fmt.Sprintf("%s_%s@%s", ln, fn, dom),
			fmt.Sprintf("%s%s@%s", ln, fn, dom),
			fmt.Sprintf("%s-%s@%s", fn, ln, dom),
			fmt.Sprintf("%s-%s@%s", fInit, ln, dom),
		}
	}

	seen := make(map[string]bool)
	var deduped []string
	for _, p := range rawPatterns {
		parts := strings.Split(p, "@")
		if len(parts) != 2 {
			continue
		}
		localPart := parts[0]

		if localPart == "" ||
			strings.HasPrefix(localPart, ".") || strings.HasPrefix(localPart, "_") || strings.HasPrefix(localPart, "-") ||
			strings.HasSuffix(localPart, ".") || strings.HasSuffix(localPart, "_") || strings.HasSuffix(localPart, "-") ||
			strings.Contains(localPart, "..") {
			continue
		}

		if !seen[p] {
			seen[p] = true
			deduped = append(deduped, p)
		}
	}

	if len(deduped) > 15 {
		return deduped[:15]
	}
	return deduped
}
