package shortcode

import (
	"math"
	"testing"
)

func TestNumberRoundTripAsSuffixGrows(t *testing.T) {
	for _, number := range []uint64{1, 2, 61*61*61 - 1, 61*61*61, math.MaxInt64} {
		code, err := Encode(number)
		if err != nil {
			t.Fatal(err)
		}

		if decoded, ok := Number(code); !ok || decoded != number {
			t.Fatalf("number %d did not round trip through %q", number, code)
		}

		if number < 61*61*61 && len(code) != 4 || number >= 61*61*61 && len(code) <= 4 {
			t.Fatalf("unexpected code length for %d: %q", number, code)
		}
	}

	if _, ok := Number("????"); ok {
		t.Fatal("invalid number accepted")
	}
}
