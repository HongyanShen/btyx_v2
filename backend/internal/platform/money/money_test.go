package money

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestParseFenRejectsInvalidOrFractionalInput(t *testing.T) {
	for _, input := range []string{"", "NaN", "1.5", "1e4", "-1", "999999999999999999999"} {
		if _, err := ParseFen(input); err == nil {
			t.Errorf("ParseFen(%q) should fail", input)
		}
	}
}

func TestMoneyHasExactArithmeticAndDisplay(t *testing.T) {
	a, err := ParseFen("750")
	if err != nil {
		t.Fatal(err)
	}

	if got := Yuan(a.Add(a)); got != "15.00" {
		t.Fatalf("got %s", got)
	}

	b, err := ParseFen("9007199254740993")
	if err != nil {
		t.Fatal(err)
	}

	if got := b.Add(a).
		String(); got != "9007199254741743" {
		t.Fatalf("precision lost: %s", got)
	}

	for _, test := range []struct {
		fen  decimal.Decimal
		want string
	}{
		{a, "7.50"},
		{b, "90071992547409.93"},
		{decimal.NewFromInt(-10), "-0.10"},
	} {
		if got := Yuan(test.fen); got != test.want {
			t.Errorf("Yuan(%s) = %q, want %q", test.fen, got, test.want)
		}
	}
}
