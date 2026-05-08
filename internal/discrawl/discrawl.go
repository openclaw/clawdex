package discrawl

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
	ChannelID     string `json:"channel_id"`
	Name          string `json:"name"`
	Messages      int    `json:"messages"`
	FirstMessage  string `json:"first_message"`
	LastMessage   string `json:"last_message"`
	CounterpartID string `json:"counterpart_id"`
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
	cmd := exec.CommandContext(ctx, binary, "-json", "file:"+dbPath+"?mode=ro&immutable=1", query)
	raw, err := cmd.Output()
	if err != nil {
		return nil, sqliteErr(err)
	}
	var rows []dmRow
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]model.SourceContact, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" || row.ChannelID == "" {
			continue
		}
		accounts := map[string][]string{"discord": {"channel:" + row.ChannelID}}
		if row.CounterpartID != "" {
			accounts["discord"] = append(accounts["discord"], "user:"+row.CounterpartID)
		}
		out = append(out, model.SourceContact{
			Source:     "discord",
			ExternalID: row.ChannelID,
			Name:       name,
			Tags:       []string{"discord", "dm"},
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
		path = filepath.Join(home, ".discrawl", "discrawl.db")
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
		return fmt.Errorf("discrawl sqlite query: %s", strings.TrimSpace(string(exitErr.Stderr)))
	}
	return err
}

const dmQuery = `
with dm as (
  select
    c.id as channel_id,
    c.name as name,
    count(m.id) as messages,
    min(m.created_at) as first_message,
    max(m.created_at) as last_message
  from channels c
  join messages m on m.channel_id = c.id
  where c.guild_id = '@me' and c.kind = 'dm'
  group by c.id, c.name
  having count(m.id) > %d
),
self as (
  select m.author_id
  from channels c
  join messages m on m.channel_id = c.id
  where c.guild_id = '@me' and c.kind = 'dm' and m.author_id is not null
  group by m.author_id
  order by count(distinct c.id) desc, count(m.id) desc
  limit 1
),
author_counts as (
  select c.id as channel_id, m.author_id, count(m.id) as messages
  from channels c
  join messages m on m.channel_id = c.id
  where c.guild_id = '@me' and c.kind = 'dm' and m.author_id is not null
  group by c.id, m.author_id
)
select
  dm.channel_id,
  dm.name,
  dm.messages,
  dm.first_message,
  dm.last_message,
  coalesce((
    select ac.author_id
    from author_counts ac, self
    where ac.channel_id = dm.channel_id and ac.author_id <> self.author_id
    order by ac.messages desc
    limit 1
  ), '') as counterpart_id
from dm
order by dm.messages desc, lower(dm.name);
`
