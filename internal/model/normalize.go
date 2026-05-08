package model

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var slugDash = regexp.MustCompile(`-+`)

func Slug(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r), r == '-', r == '_', r == '\'', r == '.':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(slugDash.ReplaceAllString(b.String(), "-"), "-")
	if out == "" {
		return "person"
	}
	return out
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizePhone(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.TrimPrefix(out, "00")
	return out
}

func NormalizeName(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(name))), " ")
}

func PathSlug(path string) string {
	return filepath.Base(filepath.Dir(path))
}
