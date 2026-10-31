package main

import (
	"os"
	"strconv"
	"strings"
)

// getenv returns the env var's trimmed value, or def when unset/blank.
func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// atoiDefault parses s as an int, returning def when empty or unparsable.
func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n == 0 {
		return def
	}
	return n
}
