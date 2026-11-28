package wxnotice

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	phrasePattern = regexp.MustCompile(`^\p{Han}{1,5}$`)
	amountPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?元$`)
)

// FieldError returns the offending WeChat keyword before a grant is consumed.
func (card Card) FieldError() string {
	for key, field := range card.Data {
		value := field.Value
		valid := false
		switch strings.TrimRight(key, "0123456789") {
		case "thing":
			valid = value != "" && utf8.RuneCountInString(value) <= 20 && strings.IndexFunc(value, unicode.IsControl) == -1
		case "phrase":
			valid = phrasePattern.MatchString(value)
		case "amount":
			digits := strings.ReplaceAll(strings.TrimSuffix(value, "元"), ".", "")
			valid = amountPattern.MatchString(value) && len(digits) <= 10
		case "time":
			_, err := time.Parse("2006-01-02 15:04", value)
			valid = err == nil
		}

		if !valid {
			return key
		}
	}

	return ""
}
