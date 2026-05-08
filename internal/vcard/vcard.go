package vcard

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/openclaw/clawdex/internal/model"
)

type Options struct {
	IncludeAvatars bool
}

func Write(w io.Writer, people []model.Person) error {
	return WriteWithOptions(w, people, Options{})
}

func WriteWithOptions(w io.Writer, people []model.Person, opts Options) error {
	for _, p := range people {
		if err := writeOne(w, p, opts); err != nil {
			return err
		}
	}
	return nil
}

func writeOne(w io.Writer, p model.Person, opts Options) error {
	lines := []string{
		"BEGIN:VCARD",
		"VERSION:4.0",
		"UID:" + escape(p.ID),
		"FN:" + escape(p.Name),
		"N:" + structuredName(p),
	}
	for _, email := range p.Emails {
		if strings.TrimSpace(email.Value) == "" {
			continue
		}
		lines = append(lines, "EMAIL"+typeParam(email.Label)+":"+escape(email.Value))
	}
	for _, phone := range p.Phones {
		if strings.TrimSpace(phone.Value) == "" {
			continue
		}
		lines = append(lines, "TEL"+typeParam(phone.Label)+":"+escape(phone.Value))
	}
	if len(p.Tags) > 0 {
		lines = append(lines, "CATEGORIES:"+escape(strings.Join(p.Tags, ",")))
	}
	if opts.IncludeAvatars && strings.TrimSpace(p.Avatar.Path) != "" {
		photo, err := photoLine(p)
		if err != nil {
			return err
		}
		if photo != "" {
			lines = append(lines, photo)
		}
	}
	lines = append(lines, "NOTE:"+escape("clawdex:"+p.ID))
	lines = append(lines, "END:VCARD")
	for _, line := range lines {
		if err := folded(w, line); err != nil {
			return err
		}
	}
	return nil
}

func photoLine(p model.Person) (string, error) {
	if filepath.IsAbs(p.Avatar.Path) {
		return "", fmt.Errorf("avatar path must be relative: %s", p.Avatar.Path)
	}
	clean := filepath.Clean(filepath.FromSlash(p.Avatar.Path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("avatar path escaped person directory: %s", p.Avatar.Path)
	}
	path := filepath.Join(filepath.Dir(p.Path), clean)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := strings.TrimSpace(p.Avatar.MIME)
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "PHOTO:data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func structuredName(p model.Person) string {
	name := strings.Fields(p.Name)
	if len(name) == 0 {
		return ";;;;"
	}
	if len(name) == 1 {
		return escape(name[0]) + ";;;;"
	}
	family := name[len(name)-1]
	given := strings.Join(name[:len(name)-1], " ")
	return escape(family) + ";" + escape(given) + ";;;"
}

func typeParam(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return ""
	}
	label = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return -1
	}, label)
	if label == "" {
		return ""
	}
	return ";TYPE=" + label
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	return s
}

func folded(w io.Writer, line string) error {
	const limit = 75
	for len(line) > limit {
		cut := limit
		for !utf8.ValidString(line[:cut]) {
			cut--
		}
		if _, err := fmt.Fprint(w, line[:cut]+"\r\n "); err != nil {
			return err
		}
		line = line[cut:]
	}
	_, err := fmt.Fprint(w, line+"\r\n")
	return err
}
