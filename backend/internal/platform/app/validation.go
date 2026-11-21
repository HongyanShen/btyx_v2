package app

import (
	"fmt"
	"time"
)

func ValidateKey(key string) error {
	if len(key) < 1 || len(key) > 128 {
		return Invalid("请提供1至128字符的幂等键")
	}

	return nil
}

func ValidDate(d string) bool {
	_, err := time.Parse("2006-01-02", d)
	return err == nil
}

func Required(v, name string) error {
	if len(v) == 0 {
		return Invalid(fmt.Sprintf("%s不能为空", name))
	}

	return nil
}
