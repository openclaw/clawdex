package apple

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/openclaw/clawdex/internal/model"
)

type Contact struct {
	Identifier string   `json:"identifier"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	FullName   string   `json:"full_name"`
	Emails     []string `json:"emails"`
	Phones     []string `json:"phones"`
}

func (c Contact) Name() string {
	if strings.TrimSpace(c.FullName) != "" {
		return strings.TrimSpace(c.FullName)
	}
	return strings.TrimSpace(strings.Join([]string{c.FirstName, c.LastName}, " "))
}

func (c Contact) SourceContact() model.SourceContact {
	out := model.SourceContact{Source: "apple", ExternalID: c.Identifier, Name: c.Name()}
	for i, email := range c.Emails {
		if strings.TrimSpace(email) != "" {
			out.Emails = append(out.Emails, model.ContactValue{Value: email, Label: "other", Source: "apple", Primary: i == 0})
		}
	}
	for i, phone := range c.Phones {
		if strings.TrimSpace(phone) != "" {
			out.Phones = append(out.Phones, model.ContactValue{Value: phone, Label: "other", Source: "apple", Primary: i == 0})
		}
	}
	return out
}

func ReadFile(path string) ([]Contact, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return Decode(f)
}

func Decode(r io.Reader) ([]Contact, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var contacts []Contact
		if err := json.Unmarshal([]byte(trimmed), &contacts); err != nil {
			return nil, err
		}
		return contacts, nil
	}
	var contacts []Contact
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var c Contact
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, scanner.Err()
}

func ToSourceContacts(contacts []Contact) []model.SourceContact {
	out := make([]model.SourceContact, 0, len(contacts))
	for _, contact := range contacts {
		if strings.TrimSpace(contact.Name()) == "" {
			continue
		}
		out = append(out, contact.SourceContact())
	}
	return out
}
