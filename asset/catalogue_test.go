package asset

import "testing"

func TestFontCatalogueCompatibility(t *testing.T) {
	if len(FontCatalogue) != 18 || !IsCatalogued("DotGothic16") || IsCatalogued("dotgothic16") {
		t.Fatalf("catalogue mismatch: %d entries", len(FontCatalogue))
	}
	for alias, family := range FontAliases {
		if alias == "" || !IsCatalogued(family) {
			t.Fatalf("invalid alias %q=%q", alias, family)
		}
	}
}

func TestFontAliasAndStackCompatibility(t *testing.T) {
	for input, want := range map[string]string{
		"pop": "Hachi Maru Pop", "POP": "Hachi Maru Pop", "dotgothic16": "DotGothic16",
	} {
		if got, ok := ResolveFontAlias(input); !ok || got != want {
			t.Fatalf("ResolveFontAlias(%q)=%q,%v want %q", input, got, ok, want)
		}
	}
	if got := ResolveFontStack(`pop, dot, sans-serif`); got != "Hachi Maru Pop, DotGothic16, sans-serif" {
		t.Fatalf("stack=%q", got)
	}
	if got := ResolveFontStack(`"Vina Sans", 'pop', serif`); got != "Vina Sans, Hachi Maru Pop, serif" {
		t.Fatalf("quoted stack=%q", got)
	}
}

func TestFontSuggestionsCompatibility(t *testing.T) {
	for input, want := range map[string]string{
		"dacing script": "Dancing Script", "Dela Gothic Oen": "Dela Gothic One",
		"Yusei Magik": "Yusei Magic", "Inconsolatta": "Inconsolata",
		"mplus rounded 1c": "M PLUS Rounded 1c", "dot gothic 16": "DotGothic16",
		"rock n roll one": "RocknRoll One", "exo2": "Exo 2",
	} {
		if got, ok := SuggestionFor(input); !ok || got != want {
			t.Fatalf("SuggestionFor(%q)=%q,%v want %q", input, got, ok, want)
		}
	}
	for _, input := range []string{"", "Helvetica", "Jiyu no Tsubasa", "zzzzzzzzzzzzzzzz"} {
		if got, ok := SuggestionFor(input); ok {
			t.Fatalf("SuggestionFor(%q)=%q unexpectedly", input, got)
		}
	}
	if reason, ok := UnavailableReason(" JIYU NO TSUBASA "); !ok || reason == "" {
		t.Fatal("missing unavailable reason")
	}
}
