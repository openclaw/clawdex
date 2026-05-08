package vcard

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/openclaw/clawdex/internal/model"
)

func Write(w io.Writer, people []model.Person) error {
	for _, p := range people {
		if err := writeOne(w, p); err != nil {
			return err
		}
	}
	return nil
}

func writeOne(w io.Writer, p model.Person) error {
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
	lines = append(lines, "NOTE:"+escape("clawdex:"+p.ID))
	lines = append(lines, "END:VCARD")
	for _, line := range lines {
		if err := folded(w, line); err != nil {
			return err
		}
	}
	return nil
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
