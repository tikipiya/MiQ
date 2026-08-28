package theme

import "testing"

func FuzzParseColorString(f *testing.F) {
	for _, seed := range []string{"#fff", "transparent", "rgb(1 2 3 / 50%)", "lab(50% 40 59.5)", "not a color", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		parsed, err := ParseColor(input)
		if err != nil {
			return
		}
		if _, roundTripErr := ParseColor(ColorHex(parsed)); roundTripErr != nil {
			t.Fatalf("round trip %q: %v", input, roundTripErr)
		}
	})
}
