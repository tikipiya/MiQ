package asset

import (
	"strings"
	"unicode"
)

var DefaultFonts = []string{"M PLUS Rounded 1c", "Noto Sans JP"}

// FontCatalogue preserves the original curated catalogue. Arbitrary Google
// Fonts families can still be installed by their full name.
var FontCatalogue = []string{
	"Noto Sans JP", "M PLUS Rounded 1c", "Dela Gothic One", "DotGothic16",
	"Hachi Maru Pop", "Rampart One", "Reggae One", "RocknRoll One",
	"Zen Old Mincho", "Yuji Syuku", "Yusei Magic", "Inconsolata", "Exo 2",
	"Bruno Ace SC", "Poltawski Nowy", "Vina Sans", "Dancing Script",
	"Castoro Titling",
}

var FontAliases = map[string]string{
	"sans": "Noto Sans JP", "mplus": "M PLUS Rounded 1c", "dela": "Dela Gothic One",
	"dot": "DotGothic16", "pop": "Hachi Maru Pop", "rampart": "Rampart One",
	"reggae": "Reggae One", "rocknroll": "RocknRoll One", "serif": "Zen Old Mincho",
	"yuji": "Yuji Syuku", "yusei": "Yusei Magic", "inconsolata": "Inconsolata",
	"exo2": "Exo 2", "bruno": "Bruno Ace SC", "poltawski": "Poltawski Nowy",
	"vina": "Vina Sans", "script": "Dancing Script", "castoro": "Castoro Titling",
}

var GenericFontFamilies = map[string]bool{
	"sans-serif": true, "serif": true, "monospace": true, "cursive": true, "fantasy": true,
}

func IsCatalogued(family string) bool {
	for _, candidate := range FontCatalogue {
		if family == candidate {
			return true
		}
	}
	return false
}

func ResolveFontAlias(input string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(input))
	if key == "" {
		return "", false
	}
	if family, ok := FontAliases[key]; ok {
		return family, true
	}
	for _, family := range FontCatalogue {
		if strings.EqualFold(key, family) {
			return family, true
		}
	}
	return "", false
}

func ResolveFontStack(stack string) string {
	parts := strings.Split(stack, ",")
	for index, part := range parts {
		family := strings.Trim(strings.TrimSpace(part), "\"'")
		if !GenericFontFamilies[family] {
			if resolved, ok := ResolveFontAlias(family); ok {
				family = resolved
			}
		}
		parts[index] = family
	}
	return strings.Join(parts, ", ")
}

func UnavailableReason(family string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(family), "Jiyu no Tsubasa") {
		return "not distributed through Google Fonts; its licence is unclear", true
	}
	return "", false
}

func SuggestionFor(family string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(family))
	explicit := map[string]string{
		"noto sans jp regular": "Noto Sans JP",
		"m+ rounded 1c":        "M PLUS Rounded 1c",
	}
	if value, ok := explicit[key]; ok {
		return value, true
	}
	query := normalizeFontName(key)
	if query == "" {
		return "", false
	}
	best, bestDistance := "", int(^uint(0)>>1)
	for _, candidate := range FontCatalogue {
		distance := levenshtein(query, normalizeFontName(candidate))
		if distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	tolerance := len(query) / 3
	if tolerance < 2 {
		tolerance = 2
	}
	return best, bestDistance <= tolerance
}

func normalizeFontName(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func levenshtein(left, right string) int {
	a, b := []rune(left), []rune(right)
	previous := make([]int, len(b)+1)
	for index := range previous {
		previous[index] = index
	}
	for i, ar := range a {
		current := make([]int, len(b)+1)
		current[0] = i + 1
		for j, br := range b {
			cost := 0
			if ar != br {
				cost = 1
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(b)]
}
