package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/openclaw/clawdex/internal/model"
)

const (
	defaultAvatarConcurrency = 4
	maxAvatarConcurrency     = 8
	maxAvatarBytes           = 10 << 20
	avatarLookupTimeout      = 20 * time.Second
)

type Options struct {
	IncludeAvatars    bool
	AvatarConcurrency int
}

type AvatarFetchFunc func(context.Context, string) (model.SourceAvatar, error)

type GogAdapter struct {
	Binary      string
	FetchAvatar AvatarFetchFunc
}

func (g GogAdapter) ListContacts(ctx context.Context, account string) ([]model.SourceContact, error) {
	return g.ListContactsWithOptions(ctx, account, Options{})
}

func (g GogAdapter) ListContactsWithOptions(ctx context.Context, account string, opts Options) ([]model.SourceContact, error) {
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
			if opts.IncludeAvatars {
				g.attachAvatars(ctx, binary, account, out, opts.avatarConcurrency())
			}
			return out, nil
		}
		page = nextPage
	}
}

func (o Options) avatarConcurrency() int {
	if o.AvatarConcurrency <= 0 {
		return defaultAvatarConcurrency
	}
	if o.AvatarConcurrency > maxAvatarConcurrency {
		return maxAvatarConcurrency
	}
	return o.AvatarConcurrency
}

func (g GogAdapter) attachAvatars(ctx context.Context, binary string, account string, contacts []model.SourceContact, concurrency int) {
	if len(contacts) == 0 {
		return
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(contacts) {
		concurrency = len(contacts)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Go(func() {
			for i := range jobs {
				g.attachAvatar(ctx, binary, account, &contacts[i])
			}
		})
	}
	for i := range contacts {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
}

func (g GogAdapter) attachAvatar(ctx context.Context, binary string, account string, contact *model.SourceContact) {
	if ctx.Err() != nil || contact == nil {
		return
	}
	if strings.TrimSpace(contact.ExternalID) == "" || contact.Avatar != nil {
		return
	}
	lookupCtx, cancel := context.WithTimeout(ctx, avatarLookupTimeout)
	defer cancel()
	raw, err := g.rawContact(lookupCtx, binary, account, contact.ExternalID)
	if err != nil {
		return
	}
	url, err := parseGogPhotoURL(raw)
	if err != nil || url == "" {
		return
	}
	avatar, err := g.fetchAvatar(lookupCtx, url)
	if err != nil || len(avatar.Data) == 0 {
		return
	}
	avatar.URL = url
	contact.Avatar = &avatar
}

func (g GogAdapter) rawContact(ctx context.Context, binary string, account string, identifier string) ([]byte, error) {
	args := []string{"--no-input", "contacts", "raw", identifier, "--person-fields", "photos", "--json"}
	if strings.TrimSpace(account) != "" {
		args = append([]string{"--account", account}, args...)
	}
	// #nosec G204 -- the adapter intentionally shells to a configured gog binary without using a shell.
	cmd := exec.CommandContext(ctx, binary, args...)
	return cmd.Output()
}

func (g GogAdapter) fetchAvatar(ctx context.Context, url string) (model.SourceAvatar, error) {
	if g.FetchAvatar != nil {
		return g.FetchAvatar(ctx, url)
	}
	return fetchAvatarURL(ctx, url)
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

type gogPhotoEnvelope struct {
	Contact *gogPhotoPerson `json:"contact"`
	Person  *gogPhotoPerson `json:"person"`
	Photos  []gogPhoto      `json:"photos"`
}

type gogPhotoPerson struct {
	Photos []gogPhoto `json:"photos"`
}

type gogPhoto struct {
	URL      string `json:"url"`
	Metadata struct {
		Primary bool `json:"primary"`
	} `json:"metadata"`
}

func parseGogPhotoURL(data []byte) (string, error) {
	var env gogPhotoEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", err
	}
	photos := env.Photos
	if len(photos) == 0 && env.Contact != nil {
		photos = env.Contact.Photos
	}
	if len(photos) == 0 && env.Person != nil {
		photos = env.Person.Photos
	}
	for _, photo := range photos {
		if photo.Metadata.Primary && strings.TrimSpace(photo.URL) != "" {
			return strings.TrimSpace(photo.URL), nil
		}
	}
	for _, photo := range photos {
		if strings.TrimSpace(photo.URL) != "" {
			return strings.TrimSpace(photo.URL), nil
		}
	}
	return "", nil
}

func fetchAvatarURL(ctx context.Context, url string) (model.SourceAvatar, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return model.SourceAvatar{}, fmt.Errorf("unsupported avatar URL: %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return model.SourceAvatar{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return model.SourceAvatar{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return model.SourceAvatar{}, fmt.Errorf("avatar fetch failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAvatarBytes+1))
	if err != nil {
		return model.SourceAvatar{}, err
	}
	if len(data) > maxAvatarBytes {
		return model.SourceAvatar{}, errors.New("avatar too large")
	}
	mime := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if idx := strings.IndexByte(mime, ';'); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	return model.SourceAvatar{Data: data, MIME: mime, URL: url}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
