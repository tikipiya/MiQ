package textlayout

import (
	"reflect"
	"testing"
	"unicode/utf8"
)

func runeWidth(value string) float64 { return float64(utf8.RuneCountInString(value)) }

func TestWrapPrefersSpaces(t *testing.T) {
	got := Wrap("alpha beta gamma", 10, runeWidth)
	want := []string{"alpha beta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wrap = %#v, want %#v", got, want)
	}
}

func TestWrapBreaksCJKByGrapheme(t *testing.T) {
	got := Wrap("文章を整える", 4, runeWidth)
	want := []string{"文章を整", "える"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wrap = %#v, want %#v", got, want)
	}
}

func TestWrapKeepsZWJClusterTogether(t *testing.T) {
	got := Wrap("A👨‍👩‍👧‍👦B", 1, func(value string) float64 {
		if value == "👨‍👩‍👧‍👦" {
			return 1
		}
		return runeWidth(value)
	})
	want := []string{"A", "👨‍👩‍👧‍👦", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wrap = %#v, want %#v", got, want)
	}
}

func TestEllipsize(t *testing.T) {
	if got, want := Ellipsize("abcdef", 4, runeWidth), "abc…"; got != want {
		t.Fatalf("Ellipsize = %q, want %q", got, want)
	}
}
