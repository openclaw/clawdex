package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/clawdex/internal/contactexport"
	"github.com/openclaw/clawdex/internal/model"
)

func TestExecuteImportContactsFromCrawler(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[{"display_name":"Ada Source","phone_numbers":[" +1 555 0100 "]}]}`)
	t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err != nil {
		t.Fatalf("import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "create\tAda Source") {
		t.Fatalf("import contacts out = %s", out.String())
	}
}

func TestExecuteImportContactsFromCrawlerPath(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[{"display_name":"Ada Path","phone_numbers":["123"]}]}`)
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "create\tAda Path") {
		t.Fatalf("import contacts out = %s", out.String())
	}
}

func TestExecuteImportContactsNoopJSONIsEmptyArray(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[{"display_name":"Ada Source","phone_numbers":["+1 555 0100"]}]}`)
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("first import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--json", "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("second import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	var changes []model.ImportChange
	if err := json.Unmarshal(out.Bytes(), &changes); err != nil {
		t.Fatalf("noop import output is not an array: %s", out.String())
	}
	if len(changes) != 0 {
		t.Fatalf("noop import changes = %#v", changes)
	}
}

func TestExecuteImportContactsDoesNotShellExpandManifestArgv(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "telecrawl")
	manifest := `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Fake Crawler","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export;echo shell-expanded"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"metadata\" ]; then\n" +
		"cat <<'JSON'\n" + manifest + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"contacts\" ] && [ \"$3\" = \"export;echo shell-expanded\" ]; then\n" +
		"cat <<'JSON'\n{\"contacts\":[{\"display_name\":\"Ada Argv\",\"phone_numbers\":[\"123\"]}]}\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"echo unexpected args: \"$@\" >&2\nexit 2\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "create\tAda Argv") {
		t.Fatalf("import contacts out = %s", out.String())
	}
}

func TestExecuteImportContactsRejectsMutatingCommand(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawlerManifest(t, "telecrawl", `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export"],"json":true,"mutates":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`, `{"contacts":[]}`)
	t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err == nil {
		t.Fatal("expected mutating command error")
	}
}

func TestExecuteImportContactsRejectsBadManifests(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
	}{
		{
			name:     "wrong schema",
			manifest: `{"schema_version":"not-crawlkit","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "missing command",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "not json",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","contacts","export"]}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "json command missing json flag",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "empty argv",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":[],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, data := testPaths(t)
			var out, errOut bytes.Buffer
			if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
				t.Fatal(err)
			}
			fake := writeFakeContactCrawlerManifest(t, "telecrawl", tc.manifest, `{"contacts":[]}`)
			t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
			out.Reset()
			errOut.Reset()
			if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err == nil {
				t.Fatal("expected bad manifest error")
			}
		})
	}
}

func TestExecuteImportContactsRejectsBadPayload(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[]} private junk`)
	t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err == nil {
		t.Fatal("expected bad payload error")
	}
}

func TestExecuteImportContactsRejectsDifferentManifestBinary(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawlerManifest(t, "telecrawl", `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["othercrawl","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`, `{"contacts":[]}`)
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", fake}, &out, &errOut); err == nil {
		t.Fatal("expected mismatched manifest binary error")
	}
}

func TestReadCrawlerManifestErrors(t *testing.T) {
	dir := t.TempDir()
	failing := filepath.Join(dir, "failing")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\necho metadata failed >&2\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := readCrawlerManifest(t.Context(), failing); err == nil {
		t.Fatal("expected metadata command error")
	}

	badJSON := filepath.Join(dir, "badjson")
	if err := os.WriteFile(badJSON, []byte("#!/bin/sh\necho not-json\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := readCrawlerManifest(t.Context(), badJSON); err == nil {
		t.Fatal("expected metadata decode error")
	}
}

func TestReadCrawlerContactsReportsExportFailure(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "telecrawl")
	manifest := `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Fake Crawler","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"metadata\" ]; then\n" +
		"cat <<'JSON'\n" + manifest + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"echo export failed >&2\nexit 9\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readCrawlerContacts(t.Context(), fake); err == nil {
		t.Fatal("expected export command error")
	}
}

func TestContactExportArgv(t *testing.T) {
	got, err := contactExportArgv("/tmp/telecrawl", []string{"telecrawl", "--json", "contacts", "export"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "/tmp/telecrawl" || got[1] != "--json" {
		t.Fatalf("argv = %#v", got)
	}
	if _, err := contactExportArgv("telecrawl", nil); err == nil {
		t.Fatal("expected empty argv error")
	}
	if _, err := contactExportArgv("telecrawl", []string{"othercrawl"}); err == nil {
		t.Fatal("expected mismatched argv error")
	}
	if _, err := contactExportArgv("telecrawl", []string{"telecrawl", "contacts", "export"}); err == nil {
		t.Fatal("expected missing json flag error")
	}
}

func TestSourceContactsFromExportMapsPhones(t *testing.T) {
	contacts := sourceContactsFromExport("telecrawl", contactexport.ContactExport{Contacts: []contactexport.Contact{{
		DisplayName:  "Ada",
		PhoneNumbers: []string{"123", "456"},
	}}})
	if len(contacts) != 1 {
		t.Fatalf("contacts = %#v", contacts)
	}
	got := contacts[0]
	if got.Source != "telecrawl" || got.Name != "Ada" || len(got.Phones) != 2 {
		t.Fatalf("mapped contact = %#v", got)
	}
	if !got.Phones[0].Primary || got.Phones[1].Primary {
		t.Fatalf("primary phones = %#v", got.Phones)
	}
	if got.Phones[1].Value != "456" || got.Phones[1].Source != "telecrawl" {
		t.Fatalf("second phone = %#v", got.Phones[1])
	}
}
