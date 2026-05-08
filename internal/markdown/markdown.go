package markdown

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openclaw/clawdex/internal/model"
	"gopkg.in/yaml.v3"
)

type RepairReport struct {
	Path              string   `json:"path"`
	Needed            bool     `json:"needed"`
	Problems          []string `json:"problems,omitempty"`
	RecoveredMetadata string   `json:"recovered_metadata,omitempty"`
}

func NewPerson(name string, now time.Time) model.Person {
	return model.Person{
		ID:        "person_" + uuid.NewString(),
		Name:      strings.TrimSpace(name),
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
}

func NewNote(personID, kind, source, body string, occurredAt, now time.Time, topics []string) model.Note {
	if occurredAt.IsZero() {
		occurredAt = now
	}
	return model.Note{
		ID:         "note_" + uuid.NewString(),
		PersonID:   personID,
		OccurredAt: occurredAt.UTC(),
		CapturedAt: now.UTC(),
		Kind:       strings.TrimSpace(kind),
		Source:     strings.TrimSpace(source),
		Confidence: "high",
		Privacy:    "normal",
		Topics:     topics,
		Body:       body,
	}
}

func ReadPerson(path string) (model.Person, RepairReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Person{}, RepairReport{}, err
	}
	front, body, ok := splitFrontmatter(data)
	report := RepairReport{Path: path}
	var p model.Person
	if ok {
		if err := yaml.Unmarshal([]byte(front), &p); err != nil {
			report.Needed = true
			report.Problems = append(report.Problems, "invalid YAML frontmatter: "+err.Error())
			report.RecoveredMetadata = front
			p = salvagePerson(front)
		}
	} else {
		report.Needed = true
		report.Problems = append(report.Problems, "missing YAML frontmatter")
		body = string(data)
	}
	p.Body = strings.TrimLeft(body, "\n")
	p.Path = path
	inferPerson(&p, path)
	return p, report, nil
}

func WritePerson(path string, p model.Person) error {
	inferPerson(&p, path)
	p.UpdatedAt = p.UpdatedAt.UTC()
	front, err := yaml.Marshal(personFrontmatter(p))
	if err != nil {
		return err
	}
	body := strings.TrimLeft(p.Body, "\n")
	if body == "" {
		body = "# " + p.Name + "\n"
	}
	return atomicWrite(path, appendFrontmatter(front, body), 0o600)
}

func RepairPerson(path, repairRoot string, p model.Person, report RepairReport, backup bool) error {
	if !report.Needed {
		return nil
	}
	if backup {
		if err := backupOriginal(path, repairRoot); err != nil {
			return err
		}
	}
	if report.RecoveredMetadata != "" && !strings.Contains(p.Body, "## Recovered metadata") {
		p.Body = strings.TrimRight(p.Body, "\n") + "\n\n## Recovered metadata\n\n```yaml\n" + strings.TrimSpace(report.RecoveredMetadata) + "\n```\n"
	}
	return WritePerson(path, p)
}

func ReadNote(path string) (model.Note, RepairReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Note{}, RepairReport{}, err
	}
	front, body, ok := splitFrontmatter(data)
	report := RepairReport{Path: path}
	var n model.Note
	if ok {
		if err := yaml.Unmarshal([]byte(front), &n); err != nil {
			report.Needed = true
			report.Problems = append(report.Problems, "invalid YAML frontmatter: "+err.Error())
			report.RecoveredMetadata = front
			n = salvageNote(front)
		}
	} else {
		report.Needed = true
		report.Problems = append(report.Problems, "missing YAML frontmatter")
		body = string(data)
	}
	n.Body = strings.TrimLeft(body, "\n")
	n.Path = path
	inferNote(&n, path)
	return n, report, nil
}

func WriteNote(path string, n model.Note) error {
	inferNote(&n, path)
	front, err := yaml.Marshal(noteFrontmatter(n))
	if err != nil {
		return err
	}
	return atomicWrite(path, appendFrontmatter(front, strings.TrimLeft(n.Body, "\n")), 0o600)
}

func splitFrontmatter(data []byte) (string, string, bool) {
	text := string(data)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return "", text, false
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	rest := normalized[4:]
	front, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		if front, ok := strings.CutSuffix(rest, "\n---"); ok {
			return front, "", true
		}
		return "", text, false
	}
	return front, body, true
}

func appendFrontmatter(front []byte, body string) []byte {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(bytes.TrimSpace(front))
	buf.WriteString("\n---\n")
	buf.WriteString(strings.TrimLeft(body, "\n"))
	if !strings.HasSuffix(buf.String(), "\n") {
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func inferPerson(p *model.Person, path string) {
	if p.ID == "" {
		p.ID = "person_" + uuid.NewString()
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = nameFromBody(p.Body)
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = strings.ReplaceAll(model.PathSlug(path), "-", " ")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = fileTime(path)
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = fileTime(path)
	}
	if p.Accounts == nil {
		p.Accounts = map[string][]string{}
	}
}

func inferNote(n *model.Note, path string) {
	if n.ID == "" {
		n.ID = "note_" + uuid.NewString()
	}
	if n.OccurredAt.IsZero() {
		n.OccurredAt = fileTime(path)
	}
	if n.CapturedAt.IsZero() {
		n.CapturedAt = fileTime(path)
	}
	if n.Kind == "" {
		n.Kind = "note"
	}
	if n.Source == "" {
		n.Source = "manual"
	}
	if n.Confidence == "" {
		n.Confidence = "medium"
	}
	if n.Privacy == "" {
		n.Privacy = "normal"
	}
}

type personFront struct {
	ID        string               `yaml:"id"`
	Name      string               `yaml:"name"`
	SortName  string               `yaml:"sort_name,omitempty"`
	Tags      []string             `yaml:"tags,omitempty"`
	Emails    []model.ContactValue `yaml:"emails,omitempty"`
	Phones    []model.ContactValue `yaml:"phones,omitempty"`
	Accounts  map[string][]string  `yaml:"accounts,omitempty"`
	Apple     *model.ExternalRef   `yaml:"apple,omitempty"`
	Google    *model.ExternalRef   `yaml:"google,omitempty"`
	CreatedAt time.Time            `yaml:"created_at"`
	UpdatedAt time.Time            `yaml:"updated_at"`
}

func personFrontmatter(p model.Person) personFront {
	return personFront{
		ID:        p.ID,
		Name:      p.Name,
		SortName:  p.SortName,
		Tags:      p.Tags,
		Emails:    p.Emails,
		Phones:    p.Phones,
		Accounts:  nonEmptyAccounts(p.Accounts),
		Apple:     nonEmptyExternal(p.Apple),
		Google:    nonEmptyExternal(p.Google),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

type noteFront struct {
	ID         string     `yaml:"id"`
	PersonID   string     `yaml:"person_id"`
	OccurredAt time.Time  `yaml:"occurred_at"`
	CapturedAt time.Time  `yaml:"captured_at"`
	Kind       string     `yaml:"kind"`
	Source     string     `yaml:"source"`
	Account    string     `yaml:"account,omitempty"`
	ExternalID string     `yaml:"external_id,omitempty"`
	Direction  string     `yaml:"direction,omitempty"`
	Confidence string     `yaml:"confidence,omitempty"`
	Topics     []string   `yaml:"topics,omitempty"`
	FollowUpAt *time.Time `yaml:"follow_up_at,omitempty"`
	Privacy    string     `yaml:"privacy,omitempty"`
}

func noteFrontmatter(n model.Note) noteFront {
	return noteFront{
		ID:         n.ID,
		PersonID:   n.PersonID,
		OccurredAt: n.OccurredAt,
		CapturedAt: n.CapturedAt,
		Kind:       n.Kind,
		Source:     n.Source,
		Account:    n.Account,
		ExternalID: n.ExternalID,
		Direction:  n.Direction,
		Confidence: n.Confidence,
		Topics:     n.Topics,
		FollowUpAt: nonZeroTime(n.FollowUpAt),
		Privacy:    n.Privacy,
	}
}

func nonEmptyExternal(ref model.ExternalRef) *model.ExternalRef {
	if ref.ID == "" && ref.Resource == "" && ref.ETag == "" && ref.LastSeenAt.IsZero() {
		return nil
	}
	return &ref
}

func nonEmptyAccounts(accounts map[string][]string) map[string][]string {
	if len(accounts) == 0 {
		return nil
	}
	return accounts
}

func nonZeroTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func nameFromBody(body string) string {
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if title, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(title)
		}
	}
	return ""
}

func fileTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UTC()
	}
	return info.ModTime().UTC()
}

func atomicWrite(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func backupOriginal(path, repairRoot string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rel := strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator))
	dest := filepath.Join(repairRoot, time.Now().UTC().Format("20060102T150405Z"), rel)
	if !strings.HasPrefix(dest, filepath.Clean(repairRoot)+string(filepath.Separator)) {
		return fmt.Errorf("repair backup escaped repair root: %s", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// #nosec G703 -- dest is constrained to repairRoot above.
	return os.WriteFile(dest, data, 0o600)
}

func salvagePerson(front string) model.Person {
	var p model.Person
	values := salvageScalars(front)
	p.ID = values["id"]
	p.Name = values["name"]
	p.SortName = values["sort_name"]
	p.Tags = splitList(values["tags"])
	p.CreatedAt = parseTime(values["created_at"])
	p.UpdatedAt = parseTime(values["updated_at"])
	return p
}

func salvageNote(front string) model.Note {
	var n model.Note
	values := salvageScalars(front)
	n.ID = values["id"]
	n.PersonID = values["person_id"]
	n.Kind = values["kind"]
	n.Source = values["source"]
	n.Account = values["account"]
	n.ExternalID = values["external_id"]
	n.Direction = values["direction"]
	n.Confidence = values["confidence"]
	n.Privacy = values["privacy"]
	n.Topics = splitList(values["topics"])
	n.OccurredAt = parseTime(values["occurred_at"])
	n.CapturedAt = parseTime(values["captured_at"])
	n.FollowUpAt = parseTime(values["follow_up_at"])
	return n
}

func salvageScalars(front string) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(front, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			out[key] = value
		}
	}
	return out
}

func splitList(value string) []string {
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func NoteFileName(n model.Note) string {
	t := n.OccurredAt.UTC()
	if t.IsZero() {
		t = time.Now().UTC()
	}
	kind := "note"
	if strings.TrimSpace(n.Kind) != "" {
		kind = model.Slug(n.Kind)
	}
	if kind == "" || kind == "person" {
		kind = "note"
	}
	return fmt.Sprintf("%s-%s.md", t.Format("2006-01-02T15-04-05Z"), kind)
}
