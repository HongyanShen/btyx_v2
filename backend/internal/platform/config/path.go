package config

import "os"

func Path() string {
	if path := os.Getenv("BTYX_CONFIG"); path != "" {
		return path
	}

	return "../.env"
}
