package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/openclaw/clawdex/internal/contactexport"
	"github.com/openclaw/clawdex/internal/model"
	"github.com/openclaw/crawlkit/control"
)

func readCrawlerContacts(ctx context.Context, binary string) (string, []model.SourceContact, error) {
	manifest, err := readCrawlerManifest(ctx, binary)
	if err != nil {
		return "", nil, err
	}
	command, ok := manifest.Commands["contact-export"]
	if !ok {
		return "", nil, fmt.Errorf("%s metadata does not advertise contact-export", binary)
	}
	if !command.JSON {
		return "", nil, fmt.Errorf("%s contact-export must advertise json output", binary)
	}
	if command.Mutates {
		return "", nil, fmt.Errorf("%s contact-export must be read-only", binary)
	}
	argv, err := contactExportArgv(binary, command.Argv)
	if err != nil {
		return "", nil, err
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) // #nosec G204 -- argv comes from the local crawler manifest and is executed without a shell.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", nil, fmt.Errorf("%s contact-export failed: %w: %s", binary, err, msg)
		}
		return "", nil, fmt.Errorf("%s contact-export failed: %w", binary, err)
	}
	export, err := contactexport.Decode(bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("%s contact-export decode failed: %w", binary, err)
	}
	source := strings.TrimSpace(manifest.ID)
	if source == "" {
		source = filepath.Base(binary)
	}
	return source, sourceContactsFromExport(source, export), nil
}

func contactExportArgv(binary string, advertised []string) ([]string, error) {
	if len(advertised) == 0 {
		return nil, fmt.Errorf("%s contact-export argv is empty", binary)
	}
	requestedName := filepath.Base(binary)
	advertisedName := filepath.Base(advertised[0])
	if requestedName != "" && advertisedName != "" && requestedName != advertisedName {
		return nil, fmt.Errorf("%s contact-export argv starts with %q, want %q", binary, advertised[0], requestedName)
	}
	if !slices.Contains(advertised, "--json") {
		return nil, fmt.Errorf("%s contact-export argv must include --json", binary)
	}
	argv := slices.Clone(advertised)
	argv[0] = binary
	return argv, nil
}

func readCrawlerManifest(ctx context.Context, binary string) (control.Manifest, error) {
	cmd := exec.CommandContext(ctx, binary, "--json", "metadata") // #nosec G204 -- binary is an explicit local crawler command.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return control.Manifest{}, fmt.Errorf("%s metadata failed: %w: %s", binary, err, msg)
		}
		return control.Manifest{}, fmt.Errorf("%s metadata failed: %w", binary, err)
	}
	var manifest control.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return control.Manifest{}, fmt.Errorf("%s metadata decode failed: %w", binary, err)
	}
	if strings.TrimSpace(manifest.SchemaVersion) != control.SchemaVersion {
		return control.Manifest{}, fmt.Errorf("%s metadata schema_version = %q, want %q", binary, manifest.SchemaVersion, control.SchemaVersion)
	}
	return manifest, nil
}

func sourceContactsFromExport(source string, export contactexport.ContactExport) []model.SourceContact {
	contacts := make([]model.SourceContact, 0, len(export.Contacts))
	for _, c := range export.Contacts {
		contact := model.SourceContact{Source: source, Name: c.DisplayName}
		for i, phone := range c.PhoneNumbers {
			contact.Phones = append(contact.Phones, model.ContactValue{Value: phone, Source: source, Primary: i == 0})
		}
		contacts = append(contacts, contact)
	}
	return contacts
}
