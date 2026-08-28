package gallery

import (
	"image"
	"image/color"
	"testing"
)

func TestCaseRegistryMatchesPublishedGallery(t *testing.T) {
	cases := Cases()
	if err := validateCaseCount(cases); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"01-themes": 5, "02-layout": 6, "03-text": 10, "04-emoji": 6,
		"05-typography": 4, "06-fonts": 18, "07-quotes": 5, "08-avatar": 8,
		"09-sizing": 5, "10-formats": 4, "11-discord": 8, "12-colors": 7,
		"13-conversation": 3, "14-misskey": 3, "15-markdown": 2, "16-twitter": 2,
	}
	got := make(map[string]int)
	files := make(map[string]bool)
	for _, testCase := range cases {
		got[testCase.Group]++
		file := testCase.Group + "/" + slug(testCase.Name) + "." + string(testCase.Format)
		if files[file] {
			t.Fatalf("duplicate output %q", file)
		}
		files[file] = true
		if testCase.Render == nil {
			t.Fatalf("%s/%s has no renderer", testCase.Group, testCase.Name)
		}
	}
	for group, count := range want {
		if got[group] != count {
			t.Errorf("%s has %d cases, want %d", group, got[group], count)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %d groups, want %d", len(got), len(want))
	}
}

func TestOfflineSelectionCoversAllLocalCases(t *testing.T) {
	selected := filterCases(Cases(), Options{Offline: true})
	if len(selected) != 69 {
		t.Fatalf("offline selected %d cases, want 69", len(selected))
	}
	for _, testCase := range selected {
		if testCase.Network {
			t.Fatalf("network case selected offline: %s/%s", testCase.Group, testCase.Name)
		}
	}
}

func TestBlockDifference(t *testing.T) {
	left := image.NewNRGBA(image.Rect(0, 0, 30, 16))
	right := image.NewNRGBA(image.Rect(0, 0, 30, 16))
	if difference := BlockDifference(left, right, 30, 16); difference != 0 {
		t.Fatalf("identical difference = %v", difference)
	}
	right.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	if difference := BlockDifference(left, right, 30, 16); difference <= 0 {
		t.Fatalf("changed difference = %v", difference)
	}
}
