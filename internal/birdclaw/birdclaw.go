package birdclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openclaw/clawdex/internal/model"
)

type Adapter struct {
	DBPath string
	Binary string
}

type dmRow struct {
	ConversationID string `json:"conversation_id"`
	ProfileID      string `json:"profile_id"`
	Handle         string `json:"handle"`
	DisplayName    string `json:"display_name"`
	Title          string `json:"title"`
	Messages       int    `json:"messages"`
	FirstMessage   string `json:"first_message"`
	LastMessage    string `json:"last_message"`
}

func (a Adapter) ListDMContacts(ctx context.Context, minMessages int) ([]model.SourceContact, error) {
	if minMessages < 1 {
		minMessages = 1
	}
	dbPath, err := resolveDBPath(a.DBPath)
	if err != nil {
		return nil, err
	}
	binary := strings.TrimSpace(a.Binary)
	if binary == "" {
		binary = "sqlite3"
	}
	query := fmt.Sprintf(dmQuery, minMessages)
	// #nosec G204 -- sqlite3 is a configured binary and all arguments are passed without a shell.
	cmd := exec.CommandContext(ctx, binary, "-json", dbPath, query)
	raw, err := cmd.Output()
	if err != nil {
		return nil, sqliteErr(err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var rows []dmRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]model.SourceContact, 0, len(rows))
	for _, row := range rows {
		name := firstNonEmpty(row.DisplayName, row.Title, row.Handle)
		if name == "" || row.ConversationID == "" {
			continue
		}
		accounts := map[string][]string{"x": {"dm:" + row.ConversationID}}
		if row.Handle != "" {
			accounts["x"] = append(accounts["x"], "@"+strings.TrimPrefix(row.Handle, "@"))
		}
		if row.ProfileID != "" {
			accounts["x"] = append(accounts["x"], "user:"+row.ProfileID)
		}
		out = append(out, model.SourceContact{
			Source:     "x",
			ExternalID: row.ConversationID,
			Name:       name,
			Tags:       []string{"x", "dm"},
			Accounts:   accounts,
		})
	}
	return out, nil
}

func resolveDBPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, ".birdclaw", "birdclaw.sqlite")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

func sqliteErr(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("birdclaw sqlite query: %s", strings.TrimSpace(string(exitErr.Stderr)))
	}
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

const dmQuery = `
select
  c.id as conversation_id,
  c.participant_profile_id as profile_id,
  coalesce(p.handle, '') as handle,
  coalesce(p.display_name, '') as display_name,
  c.title as title,
  count(m.id) as messages,
  min(m.created_at) as first_message,
  max(m.created_at) as last_message
from dm_conversations c
join dm_messages m on m.conversation_id = c.id
left join profiles p on p.id = c.participant_profile_id
group by c.id, c.participant_profile_id, p.handle, p.display_name, c.title
having count(m.id) > %d
order by messages desc, lower(coalesce(nullif(p.display_name, ''), nullif(c.title, ''), p.handle));
`
