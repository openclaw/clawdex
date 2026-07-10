package vcard

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/openclaw/clawdex/internal/model"
	"github.com/openclaw/clawdex/internal/safefile"
)

type Options struct {
	IncludeAvatars bool
	RepoRoot       string
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

func WriteFile(path string, people []model.Person, opts Options) error {
	root, relative, err := validatedOutput(path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return safefile.AtomicWriteRoot(root, relative, 0o600, func(w io.Writer) error {
		return WriteWithOptions(w, people, opts)
	})
}

// ValidateOutputPath applies WriteFile's destination checks without writing.
func ValidateOutputPath(path string) error {
	root, _, err := validatedOutput(path)
	if err != nil {
		return err
	}
	return root.Close()
}

func validatedOutput(path string) (*os.Root, string, error) {
	if path == "" || os.IsPathSeparator(path[len(path)-1]) {
		return nil, "", fmt.Errorf("vCard output path must name a file: %s", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	parent := filepath.Dir(abs)
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, "", err
	}
	relative := filepath.Base(abs)
	if err := safefile.ValidateWriteRoot(root, relative); err != nil {
		_ = root.Close()
		return nil, "", err
	}
	return root, relative, nil
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
		photo, err := photoLine(p, opts.RepoRoot)
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

func photoLine(p model.Person, root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("contacts repo root is required when exporting avatars")
	}
	if filepath.IsAbs(p.Avatar.Path) {
		return "", fmt.Errorf("avatar path must be relative: %s", p.Avatar.Path)
	}
	clean := filepath.Clean(filepath.FromSlash(p.Avatar.Path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("avatar path escaped person directory: %s", p.Avatar.Path)
	}
	path, err := safefile.Relative(root, filepath.Join(filepath.Dir(p.Path), clean))
	if err != nil {
		return "", err
	}
	data, err := safefile.ReadFile(root, path)
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
