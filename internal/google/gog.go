package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/openclaw/clawdex/internal/model"
)

type GogAdapter struct {
	Binary string
}

func (g GogAdapter) ListContacts(ctx context.Context, account string) ([]model.SourceContact, error) {
	binary := g.Binary
	if binary == "" {
		binary = "gog"
	}
	var out []model.SourceContact
	page := ""
	for {
		args := []string{"--no-input", "contacts", "list", "--json", "--max", "1000"}
		if page != "" {
			args = append(args, "--page", page)
		}
		if strings.TrimSpace(account) != "" {
			args = append([]string{"--account", account}, args...)
		}
		// #nosec G204 -- the adapter intentionally shells to a configured gog binary without using a shell.
		cmd := exec.CommandContext(ctx, binary, args...)
		raw, err := cmd.Output()
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return nil, fmt.Errorf("gog contacts list: %s", strings.TrimSpace(string(ee.Stderr)))
			}
			return nil, err
		}
		contacts, nextPage, err := parseGogContactsPage(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, contacts...)
		if nextPage == "" {
			return out, nil
		}
		page = nextPage
	}
}

type gogEnvelope struct {
	Contacts      []gogPerson `json:"contacts"`
	Results       []gogPerson `json:"results"`
	People        []gogPerson `json:"people"`
	NextPageToken string      `json:"nextPageToken"`
}

type gogPerson struct {
	ResourceName   string   `json:"resourceName"`
	Resource       string   `json:"resource"`
	ETag           string   `json:"etag"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Phone          string   `json:"phone"`
	Emails         []string `json:"emails"`
	Phones         []string `json:"phones"`
	EmailAddresses []struct {
		Value string `json:"value"`
		Type  string `json:"type"`
	} `json:"emailAddresses"`
	PhoneNumbers []struct {
		Value string `json:"value"`
		Type  string `json:"type"`
	} `json:"phoneNumbers"`
	Names []struct {
		DisplayName string `json:"displayName"`
		GivenName   string `json:"givenName"`
		FamilyName  string `json:"familyName"`
	} `json:"names"`
}

func parseGogContacts(data []byte) ([]model.SourceContact, error) {
	contacts, _, err := parseGogContactsPage(data)
	return contacts, err
}

func parseGogContactsPage(data []byte) ([]model.SourceContact, string, error) {
	var env gogEnvelope
	if err := json.Unmarshal(data, &env); err == nil {
		people := make([]gogPerson, 0, len(env.Contacts)+len(env.Results)+len(env.People))
		people = append(people, env.Contacts...)
		people = append(people, env.Results...)
		people = append(people, env.People...)
		if len(people) > 0 {
			return convertPeople(people), env.NextPageToken, nil
		}
	}
	var people []gogPerson
	if err := json.Unmarshal(data, &people); err != nil {
		return nil, "", err
	}
	return convertPeople(people), "", nil
}

func convertPeople(people []gogPerson) []model.SourceContact {
	out := make([]model.SourceContact, 0, len(people))
	for _, p := range people {
		name := p.Name
		if name == "" && len(p.Names) > 0 {
			name = p.Names[0].DisplayName
			if name == "" {
				name = strings.TrimSpace(p.Names[0].GivenName + " " + p.Names[0].FamilyName)
			}
		}
		c := model.SourceContact{Source: "google", ExternalID: firstNonEmpty(p.ResourceName, p.Resource), Name: name, ETag: p.ETag}
		for i, email := range append(p.Emails, p.Email) {
			if strings.TrimSpace(email) != "" {
				c.Emails = append(c.Emails, model.ContactValue{Value: email, Source: "google", Primary: i == 0})
			}
		}
		for _, email := range p.EmailAddresses {
			if strings.TrimSpace(email.Value) != "" {
				c.Emails = append(c.Emails, model.ContactValue{Value: email.Value, Label: email.Type, Source: "google"})
			}
		}
		for i, phone := range append(p.Phones, p.Phone) {
			if strings.TrimSpace(phone) != "" {
				c.Phones = append(c.Phones, model.ContactValue{Value: phone, Source: "google", Primary: i == 0})
			}
		}
		for _, phone := range p.PhoneNumbers {
			if strings.TrimSpace(phone.Value) != "" {
				c.Phones = append(c.Phones, model.ContactValue{Value: phone.Value, Label: phone.Type, Source: "google"})
			}
		}
		if strings.TrimSpace(c.Name) != "" {
			out = append(out, c)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
