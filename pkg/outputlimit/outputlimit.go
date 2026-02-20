package outputlimit

import (
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	DefaultMaxBytes = 65536
	Suffix          = "\n...[truncated]"
)

func MaxBytesFromEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func Truncate(s string, maxBytes int) (out string, truncated bool) {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s, false
	}

	b := []byte(s[:maxBytes])
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	if len(b) == 0 {
		return Suffix, true
	}
	return string(b) + Suffix, true
}
