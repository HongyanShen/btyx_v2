package shortcode

import (
	"sync"

	"github.com/sqids/sqids-go"
)

var encoder = sync.OnceValues(func() (*sqids.Sqids, error) {
	return sqids.New(sqids.Options{MinLength: 4})
})

func Encode(number uint64) (string, error) {
	codec, err := encoder()
	if err != nil {
		return "", err
	}

	return codec.Encode([]uint64{number})
}

// Number accepts only the canonical spelling emitted by Encode.
func Number(code string) (uint64, bool) {
	codec, err := encoder()
	if err != nil || len(code) < 4 || len(code) > 12 {
		return 0, false
	}

	numbers := codec.Decode(code)
	if len(numbers) != 1 {
		return 0, false
	}

	canonical, err := Encode(numbers[0])
	return numbers[0], err == nil && code == canonical
}
