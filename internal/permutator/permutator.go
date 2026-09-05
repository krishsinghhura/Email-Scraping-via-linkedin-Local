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

func DetectPattern(email, firstName, lastName string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(email)), "@")
	if len(parts) != 2 {
		return "{first}.{last}"
	}
	local := parts[0]
	fn := strings.ToLower(strings.TrimSpace(firstName))
	ln := strings.ToLower(strings.TrimSpace(lastName))

	if fn == "" {
		return "{first}.{last}"
	}
	fInit := string(fn[0])
	lInit := ""
	if ln != "" {
		lInit = string(ln[0])
	}

	if ln == "" {
		if local == fn {
			return "{first}"
		}
		return "{first}.{last}"
	}

	switch local {
	case fn + "." + ln:
		return "{first}.{last}"
	case fn:
		return "{first}"
	case fn + ln:
		return "{first}{last}"
	case fInit + ln:
		return "{finit}{last}"
	case fInit + "." + ln:
		return "{finit}.{last}"
	case fn + "_" + ln:
		return "{first}_{last}"
	case ln:
		return "{last}"
	case ln + "." + fn:
		return "{last}.{first}"
	case ln + fInit:
		return "{last}{finit}"
	case fn + lInit:
		return "{first}{linit}"
	case fn + "." + lInit:
		return "{first}.{linit}"
	case ln + "_" + fn:
		return "{last}_{first}"
	case ln + fn:
		return "{last}{fn}"
	case fn + "-" + ln:
		return "{first}-{last}"
	case fInit + "-" + ln:
		return "{finit}-{last}"
	default:
		return "{first}.{last}"
	}
}

func FormatPattern(pattern, firstName, lastName, domain string) string {
	fn := strings.ToLower(strings.TrimSpace(firstName))
	ln := strings.ToLower(strings.TrimSpace(lastName))
	dom := strings.ToLower(strings.TrimSpace(domain))

	if fn == "" || dom == "" {
		return ""
	}

	fInit := string(fn[0])
	lInit := ""
	if ln != "" {
		lInit = string(ln[0])
	}

	if ln == "" {
		return fmt.Sprintf("%s@%s", fn, dom)
	}

	if pattern == "" {
		pattern = "{first}.{last}"
	}

	local := pattern
	local = strings.ReplaceAll(local, "{first}", fn)
	local = strings.ReplaceAll(local, "{last}", ln)
	local = strings.ReplaceAll(local, "{finit}", fInit)
	local = strings.ReplaceAll(local, "{linit}", lInit)

	return fmt.Sprintf("%s@%s", local, dom)
}

func GetMostLikelyCandidate(firstName, lastName, domain string) string {
	return FormatPattern("{first}.{last}", firstName, lastName, domain)
}
