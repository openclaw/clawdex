package index

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openclaw/clawdex/internal/markdown"
	"github.com/openclaw/clawdex/internal/model"
)

func (s Store) ImportContacts(source string, contacts []model.SourceContact, dryRun bool, now time.Time) ([]model.ImportChange, error) {
	people, err := s.People()
	if err != nil {
		return nil, err
	}
	var changes []model.ImportChange
	for _, contact := range contacts {
		contact.Source = source
		if strings.TrimSpace(contact.Name) == "" {
			continue
		}
		idx := matchContact(people, contact)
		if idx < 0 {
			p := markdown.NewPerson(contact.Name, now)
			p.Tags = cleanList(contact.Tags)
			p.Emails = sourceValues(contact.Emails, source)
			p.Phones = sourceValues(contact.Phones, source)
			p.Accounts = cleanAccounts(contact.Accounts)
			setExternal(&p, source, contact, now)
			change := model.ImportChange{Action: "create", PersonID: p.ID, Name: p.Name, Source: contact}
			if !dryRun {
				created, err := s.createImportedPerson(p)
				if err != nil {
					return nil, err
				}
				change.PersonID = created.ID
				change.Path = created.Path
				people = append(people, created)
			}
			changes = append(changes, change)
			continue
		}
		p := people[idx]
		beforeEmails := len(p.Emails)
		beforePhones := len(p.Phones)
		beforeTags := append([]string(nil), p.Tags...)
		beforeAccounts := cloneAccounts(p.Accounts)
		beforeApple := p.Apple
		beforeGoogle := p.Google
		p.Tags = appendMissingStrings(p.Tags, contact.Tags)
		p.Emails = appendMissingValues(p.Emails, contact.Emails, source)
		p.Phones = appendMissingValues(p.Phones, contact.Phones, source)
		p.Accounts = mergeAccounts(p.Accounts, contact.Accounts)
		setExternal(&p, source, contact, now)
		externalChanged := p.Apple != beforeApple || p.Google != beforeGoogle
		tagsChanged := strings.Join(beforeTags, "\x00") != strings.Join(p.Tags, "\x00")
		accountsChanged := !accountsEqual(beforeAccounts, p.Accounts)
		if len(p.Emails) == beforeEmails && len(p.Phones) == beforePhones && !tagsChanged && !accountsChanged && !externalChanged {
			continue
		}
		change := model.ImportChange{Action: "update", PersonID: p.ID, Name: p.Name, Source: contact, Path: p.Path}
		if !dryRun {
			p.UpdatedAt = now.UTC()
			if err := markdown.WritePerson(p.Path, p); err != nil {
				return nil, err
			}
			people[idx] = p
		}
		changes = append(changes, change)
	}
	if !dryRun {
		return changes, s.Rebuild()
	}
	return changes, nil
}

func matchContact(people []model.Person, contact model.SourceContact) int {
	for i, p := range people {
		if accountsOverlap(p.Accounts, contact.Accounts) {
			return i
		}
	}
	for i, p := range people {
		switch contact.Source {
		case "apple":
			if contact.ExternalID != "" && p.Apple.ID == contact.ExternalID {
				return i
			}
		case "google":
			if contact.ExternalID != "" && p.Google.Resource == contact.ExternalID {
				return i
			}
		}
	}
	for i, p := range people {
		for _, email := range contact.Emails {
			if model.NormalizeEmail(email.Value) != "" && personHasEmail(p, model.NormalizeEmail(email.Value)) {
				return i
			}
		}
	}
	for i, p := range people {
		for _, phone := range contact.Phones {
			if model.NormalizePhone(phone.Value) != "" && personHasPhone(p, model.NormalizePhone(phone.Value)) {
				return i
			}
		}
	}
	for i, p := range people {
		if model.NormalizeName(p.Name) != "" && model.NormalizeName(p.Name) == model.NormalizeName(contact.Name) {
			return i
		}
	}
	return -1
}

func sourceValues(values []model.ContactValue, source string) []model.ContactValue {
	out := make([]model.ContactValue, 0, len(values))
	for i, value := range values {
		value.Source = source
		if value.Label == "" {
			value.Label = "other"
		}
		if i == 0 {
			value.Primary = true
		}
		out = append(out, value)
	}
	return out
}

func cleanAccounts(accounts map[string][]string) map[string][]string {
	if len(accounts) == 0 {
		return nil
	}
	out := map[string][]string{}
	for service, values := range accounts {
		service = strings.TrimSpace(strings.ToLower(service))
		if service == "" {
			continue
		}
		cleaned := cleanList(values)
		if len(cleaned) > 0 {
			out[service] = cleaned
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneAccounts(accounts map[string][]string) map[string][]string {
	if len(accounts) == 0 {
		return nil
	}
	out := make(map[string][]string, len(accounts))
	for service, values := range accounts {
		out[service] = append([]string(nil), values...)
	}
	return out
}

func mergeAccounts(existing map[string][]string, incoming map[string][]string) map[string][]string {
	if len(incoming) == 0 {
		return existing
	}
	if existing == nil {
		existing = map[string][]string{}
	}
	for service, values := range cleanAccounts(incoming) {
		existing[service] = appendMissingStrings(existing[service], values)
	}
	return existing
}

func appendMissingStrings(existing []string, incoming []string) []string {
	seen := map[string]bool{}
	for _, value := range existing {
		seen[strings.ToLower(strings.TrimSpace(value))] = true
	}
	for _, value := range incoming {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		existing = append(existing, value)
		seen[key] = true
	}
	sort.Strings(existing)
	return existing
}

func accountsOverlap(existing map[string][]string, incoming map[string][]string) bool {
	for service, values := range cleanAccounts(incoming) {
		current := existing[service]
		for _, value := range values {
			for _, cur := range current {
				if strings.EqualFold(strings.TrimSpace(cur), strings.TrimSpace(value)) {
					return true
				}
			}
		}
	}
	return false
}

func accountsEqual(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for service, av := range a {
		bv := b[service]
		if len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
	}
	return true
}

func appendMissingValues(existing []model.ContactValue, incoming []model.ContactValue, source string) []model.ContactValue {
	for _, value := range incoming {
		key := model.NormalizeEmail(value.Value)
		if key == "" {
			key = model.NormalizePhone(value.Value)
		}
		if key == "" {
			continue
		}
		found := false
		for _, cur := range existing {
			curKey := model.NormalizeEmail(cur.Value)
			if curKey == "" {
				curKey = model.NormalizePhone(cur.Value)
			}
			if curKey == key {
				found = true
				break
			}
		}
		if !found {
			value.Source = source
			if value.Label == "" {
				value.Label = "other"
			}
			existing = append(existing, value)
		}
	}
	return existing
}

func setExternal(p *model.Person, source string, contact model.SourceContact, now time.Time) {
	switch source {
	case "apple":
		if contact.ExternalID == "" {
			return
		}
		p.Apple.ID = contact.ExternalID
		p.Apple.LastSeenAt = now.UTC()
	case "google":
		if contact.ExternalID == "" && contact.ETag == "" {
			return
		}
		p.Google.Resource = contact.ExternalID
		p.Google.ETag = contact.ETag
		p.Google.LastSeenAt = now.UTC()
	}
}

func (s Store) createImportedPerson(p model.Person) (model.Person, error) {
	dir, err := s.uniquePersonDir(model.Slug(p.Name))
	if err != nil {
		return model.Person{}, err
	}
	p.Path = filepath.Join(dir, "person.md")
	p.Body = "# " + p.Name + "\n"
	if err := markdown.WritePerson(p.Path, p); err != nil {
		return model.Person{}, err
	}
	return p, nil
}
