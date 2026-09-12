package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/clawdex/internal/model"
)

func TestParseGogContactsEnvelopeAndArray(t *testing.T) {
	inputs := [][]byte{
		[]byte(`{"contacts":[{"resourceName":"people/c1","etag":"e1","name":"Ada","email":"ada@example.com","phone":"+1 555 0100"}]}`),
		[]byte(`[{"resource":"people/c1","names":[{"displayName":"Ada"}],"emailAddresses":[{"value":"ada@example.com","type":"home"}],"phoneNumbers":[{"value":"+1","type":"mobile"}]}]`),
	}
	for _, input := range inputs {
		contacts, _, err := parseGogContactsPage(input)
		if err != nil {
			t.Fatal(err)
		}
		if len(contacts) != 1 || contacts[0].Source != "google" || contacts[0].Name != "Ada" {
			t.Fatalf("contacts = %#v", contacts)
		}
		if contacts[0].ExternalID != "people/c1" || len(contacts[0].Emails) == 0 {
			t.Fatalf("bad contact = %#v", contacts[0])
		}
	}
}

func TestGogAdapterListContactsUsesNoInput(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gog")
	if runtime.GOOS == "windows" {
		bin += ".bat"
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"" + filepath.Join(dir, "args") + "\"\ncase \"$*\" in *next*) printf '%s\\n' '{\"contacts\":[{\"resourceName\":\"people/c2\",\"name\":\"Grace\"}]}' ;; *) printf '%s\\n' '{\"contacts\":[{\"resourceName\":\"people/c1\",\"name\":\"Ada\"}],\"nextPageToken\":\"next\"}' ;; esac\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts, err := (GogAdapter{Binary: bin}).ListContactsWithOptions(t.Context(), "ada@example.com", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 2 {
		t.Fatalf("contacts = %#v", contacts)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--no-input") || !strings.Contains(string(args), "--account") {
		t.Fatalf("args = %s", args)
	}
	if !strings.Contains(string(args), "--page") {
		t.Fatalf("missing page args = %s", args)
	}
}

func writePaginationFixture(t *testing.T, maxCalls int, response string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "gog")
	if runtime.GOOS == "windows" {
		bin += ".bat"
	}
	// Bound buggy pagination by call count, independent of subprocess scheduling.
	script := fmt.Sprintf(`#!/bin/sh
if [ -f "$0.count" ]; then read -r n < "$0.count"; else n=0; fi
n=$((n+1))
printf '%%s\n' "$n" > "$0.count"
if [ "$n" -gt %d ]; then
  printf 'unexpected pagination call: %%s\n' "$n" >&2
  exit 1
fi
%s
`, maxCalls, response)
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, bin + ".count"
}

func TestGogAdapterListContactsRejectsRepeatedPageToken(t *testing.T) {
	bin, count := writePaginationFixture(t, 2, `printf '%s\n' '{"contacts":[{"resourceName":"people/c1","name":"Ada"}],"nextPageToken":"same"}'`)
	_, err := (GogAdapter{Binary: bin}).ListContactsWithOptions(t.Context(), "ada@example.com", Options{})
	if err == nil || !strings.Contains(err.Error(), `repeated nextPageToken "same"`) {
		t.Fatalf("err = %v", err)
	}
	got, readErr := os.ReadFile(count)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if n := strings.TrimSpace(string(got)); n != "2" {
		t.Fatalf("invocations = %q want 2", n)
	}
}

func TestGogAdapterListContactsRejectsAlternatingPageTokens(t *testing.T) {
	bin, count := writePaginationFixture(t, 3, `if [ $((n % 2)) -eq 1 ]; then tok=A; else tok=B; fi
printf '%s\n' "{\"contacts\":[{\"resourceName\":\"people/c$n\",\"name\":\"Ada$n\"}],\"nextPageToken\":\"$tok\"}"`)
	_, err := (GogAdapter{Binary: bin}).ListContactsWithOptions(t.Context(), "ada@example.com", Options{})
	if err == nil || !strings.Contains(err.Error(), `repeated nextPageToken "A"`) {
		t.Fatalf("err = %v", err)
	}
	got, readErr := os.ReadFile(count)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if n := strings.TrimSpace(string(got)); n != "3" {
		t.Fatalf("invocations = %q want 3", n)
	}
}

func TestGogAdapterListContactsCapsIncrementingPageTokens(t *testing.T) {
	bin, count := writePaginationFixture(t, 500, `printf '%s\n' "{\"contacts\":[{\"resourceName\":\"people/c$n\",\"name\":\"Ada$n\"}],\"nextPageToken\":\"page-$n\"}"`)
	_, err := (GogAdapter{Binary: bin}).ListContactsWithOptions(t.Context(), "ada@example.com", Options{})
	if err == nil || !strings.Contains(err.Error(), "exceeded 500 pages") {
		t.Fatalf("err = %v", err)
	}
	got, readErr := os.ReadFile(count)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if n := strings.TrimSpace(string(got)); n != "500" {
		t.Fatalf("invocations = %q want 500", n)
	}
}

func TestGogAdapterListContactsCompletesAfterFiftyPages(t *testing.T) {
	bin, count := writePaginationFixture(t, 51, `if [ "$n" -ge 51 ]; then
  printf '%s\n' "{\"contacts\":[{\"resourceName\":\"people/c$n\",\"name\":\"Ada$n\"}]}"
else
  printf '%s\n' "{\"contacts\":[{\"resourceName\":\"people/c$n\",\"name\":\"Ada$n\"}],\"nextPageToken\":\"page-$n\"}"
fi`)
	contacts, err := (GogAdapter{Binary: bin}).ListContactsWithOptions(t.Context(), "ada@example.com", Options{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(contacts) != 51 {
		t.Fatalf("contacts = %d want 51", len(contacts))
	}
	got, err := os.ReadFile(count)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.TrimSpace(string(got)); n != "51" {
		t.Fatalf("invocations = %q want 51", n)
	}
}

func TestGogAdapterListContactsWithAvatars(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gog")
	if runtime.GOOS == "windows" {
		bin += ".bat"
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"" + filepath.Join(dir, "args") + "\"\ncase \"$*\" in *\"contacts raw\"*) printf '%s\\n' '{\"photos\":[{\"url\":\"https://example.com/secondary.jpg\"},{\"url\":\"https://example.com/grace.jpg\",\"metadata\":{\"primary\":true}}]}' ;; *) printf '%s\\n' '{\"contacts\":[{\"resourceName\":\"people/g1\",\"name\":\"Grace\",\"email\":\"grace@example.com\"},{\"name\":\"No Resource\"}]}' ;; esac\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	var fetched string
	adapter := GogAdapter{
		Binary: bin,
		FetchAvatar: func(_ context.Context, url string) (model.SourceAvatar, error) {
			fetched = url
			return model.SourceAvatar{Data: []byte("avatar"), MIME: "image/jpeg"}, nil
		},
	}
	contacts, err := adapter.ListContactsWithOptions(t.Context(), "ada@example.com", Options{IncludeAvatars: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 2 {
		t.Fatalf("contacts = %#v", contacts)
	}
	if contacts[0].Avatar == nil || string(contacts[0].Avatar.Data) != "avatar" {
		t.Fatalf("avatar = %#v", contacts[0].Avatar)
	}
	if contacts[1].Avatar != nil {
		t.Fatalf("unexpected avatar for missing resource = %#v", contacts[1].Avatar)
	}
	if fetched != "https://example.com/grace.jpg" {
		t.Fatalf("fetched = %q", fetched)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "contacts\nraw\npeople/g1\n--person-fields\nphotos") {
		t.Fatalf("missing raw photo args = %s", args)
	}
}

func TestGogAdapterAvatarFailuresAreBestEffort(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gog")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncase \"$*\" in *badraw*) exit 5 ;; *badjson*) printf '%s\\n' '{' ;; *empty*) printf '%s\\n' '{}' ;; *) printf '%s\\n' '{\"photos\":[{\"url\":\"https://example.com/avatar.jpg\"}]}' ;; esac\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts := []model.SourceContact{
		{Source: "google", ExternalID: "people/badraw", Name: "Bad Raw"},
		{Source: "google", ExternalID: "people/badjson", Name: "Bad JSON"},
		{Source: "google", ExternalID: "people/empty", Name: "Empty"},
		{Source: "google", ExternalID: "people/fetcherr", Name: "Fetch Err"},
		{Source: "google", ExternalID: "people/ok", Name: "OK"},
	}
	adapter := GogAdapter{
		Binary: bin,
		FetchAvatar: func(_ context.Context, url string) (model.SourceAvatar, error) {
			if strings.Contains(url, "avatar.jpg") {
				return model.SourceAvatar{}, os.ErrPermission
			}
			return model.SourceAvatar{Data: []byte("avatar")}, nil
		},
	}
	adapter.attachAvatars(t.Context(), bin, "", contacts, 1)
	for _, contact := range contacts {
		if contact.Avatar != nil {
			t.Fatalf("unexpected avatar after failures = %#v", contacts)
		}
	}
}

func TestAvatarConcurrencyGuards(t *testing.T) {
	for _, tc := range []struct {
		options Options
		want    int
	}{
		{options: Options{}, want: defaultAvatarConcurrency},
		{options: Options{AvatarConcurrency: -1}, want: defaultAvatarConcurrency},
		{options: Options{AvatarConcurrency: 2}, want: 2},
		{options: Options{AvatarConcurrency: maxAvatarConcurrency + 1}, want: maxAvatarConcurrency},
	} {
		if got := tc.options.avatarConcurrency(); got != tc.want {
			t.Fatalf("avatarConcurrency(%#v) = %d want %d", tc.options, got, tc.want)
		}
	}
}

func TestAttachAvatarsContextGuards(t *testing.T) {
	adapter := GogAdapter{}
	adapter.attachAvatars(t.Context(), "gog", "", nil, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	contacts := []model.SourceContact{{Source: "google", ExternalID: "people/1", Name: "Canceled"}}
	adapter.attachAvatars(ctx, "gog", "", contacts, 0)
	if contacts[0].Avatar != nil {
		t.Fatalf("unexpected avatar with canceled context = %#v", contacts[0].Avatar)
	}
}

func TestGogAdapterListContactsCommandFailure(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gog")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho nope >&2\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := (GogAdapter{Binary: bin}).ListContactsWithOptions(t.Context(), "", Options{})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
	if _, err := (GogAdapter{Binary: filepath.Join(dir, "missing")}).ListContactsWithOptions(t.Context(), "", Options{}); err == nil {
		t.Fatal("expected missing binary error")
	}
}

func TestParseGogContactsRejectsInvalidJSON(t *testing.T) {
	if _, _, err := parseGogContactsPage([]byte(`{`)); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("err = %v", err)
	}
	var p gogPerson
	if err := json.Unmarshal([]byte(`{"names":[{"givenName":"Ada","familyName":"Lovelace"}]}`), &p); err != nil {
		t.Fatal(err)
	}
	got := convertPeople([]gogPerson{p})
	if len(got) != 1 || got[0].Name != "Ada Lovelace" {
		t.Fatalf("got = %#v", got)
	}
}

func TestParseGogPhotoURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
		want string
	}{
		{name: "direct primary", json: `{"photos":[{"url":"https://example.com/a.jpg"},{"url":"https://example.com/b.jpg","metadata":{"primary":true}}]}`, want: "https://example.com/b.jpg"},
		{name: "contact wrapper", json: `{"contact":{"photos":[{"url":"https://example.com/c.jpg"}]}}`, want: "https://example.com/c.jpg"},
		{name: "person wrapper", json: `{"person":{"photos":[{"url":"  https://example.com/d.jpg  "}]}}`, want: "https://example.com/d.jpg"},
		{name: "none", json: `{}`, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGogPhotoURL([]byte(tc.json))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	if _, err := parseGogPhotoURL([]byte(`{`)); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestFetchAvatarURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/too-large" {
			_, _ = w.Write(make([]byte, maxAvatarBytes+1))
			return
		}
		w.Header().Set("Content-Type", "image/png; charset=utf-8")
		_, _ = w.Write([]byte("png"))
	}))
	defer server.Close()

	avatar, err := fetchAvatarURL(t.Context(), server.URL+"/avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	if string(avatar.Data) != "png" || avatar.MIME != "image/png" || !strings.HasSuffix(avatar.URL, "/avatar.png") {
		t.Fatalf("avatar = %#v", avatar)
	}
	if _, err := fetchAvatarURL(t.Context(), "file:///tmp/avatar.png"); err == nil {
		t.Fatal("expected unsupported URL error")
	}
	if _, err := fetchAvatarURL(t.Context(), "http://[::1"); err == nil {
		t.Fatal("expected bad URL error")
	}
	if _, err := fetchAvatarURL(t.Context(), server.URL+"/missing"); err == nil {
		t.Fatal("expected non-2xx error")
	}
	if _, err := fetchAvatarURL(t.Context(), server.URL+"/too-large"); err == nil {
		t.Fatal("expected too large error")
	}
	if avatar, err := (GogAdapter{}).fetchAvatar(t.Context(), server.URL+"/avatar.png"); err != nil || string(avatar.Data) != "png" {
		t.Fatalf("default fetchAvatar = %#v err=%v", avatar, err)
	}
}

func TestFetchAvatarURLClientTimeout(t *testing.T) {
	prev := avatarHTTPClient
	avatarHTTPClient = &http.Client{Timeout: 50 * time.Millisecond}
	t.Cleanup(func() { avatarHTTPClient = prev })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("ResponseWriter does not support hijacking")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		t.Cleanup(func() { _ = conn.Close() })
	}))
	t.Cleanup(server.Close)

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := fetchAvatarURL(context.Background(), server.URL+"/stall")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected timeout error from stalled avatar fetch")
		}
		if !strings.Contains(err.Error(), "Timeout") && !strings.Contains(err.Error(), "deadline") {
			t.Fatalf("err = %v", err)
		}
		elapsed := time.Since(start)
		if elapsed >= 2*time.Second {
			t.Fatalf("stalled fetch returned after %v, want client timeout well under 2s: %v", elapsed, err)
		}
		t.Logf("stalled fetch returned in %v: %v", elapsed, err)
	case <-time.After(2 * time.Second):
		t.Fatal("fetchAvatarURL hung for 2s; client timeout did not fire")
	}
}
