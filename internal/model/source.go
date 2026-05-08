package model

type SourceContact struct {
	Source     string              `json:"source"`
	ExternalID string              `json:"external_id,omitempty"`
	Name       string              `json:"name"`
	Tags       []string            `json:"tags,omitempty"`
	Emails     []ContactValue      `json:"emails,omitempty"`
	Phones     []ContactValue      `json:"phones,omitempty"`
	Accounts   map[string][]string `json:"accounts,omitempty"`
	ETag       string              `json:"etag,omitempty"`
}

type ImportChange struct {
	Action   string        `json:"action"`
	PersonID string        `json:"person_id,omitempty"`
	Name     string        `json:"name"`
	Source   SourceContact `json:"source"`
	Path     string        `json:"path,omitempty"`
}
