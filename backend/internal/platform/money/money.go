package money

import (
	"errors"
	"regexp"

	"github.com/shopspring/decimal"
)

var digits = regexp.MustCompile(`^(0|[1-9][0-9]{0,17})$`)

func ParseFen(s string) (decimal.Decimal, error) {
	if !digits.MatchString(s) {
		return decimal.Zero, errors.New("金额必须是非负整数分，最多18位")
	}

	return decimal.NewFromString(s)
}

func Yuan(fen decimal.Decimal) string {
	return fen.Shift(-2).
		StringFixed(2)
}
