package model

type SourceContact struct {
	Source     string              `json:"source"`
	ExternalID string              `json:"external_id,omitempty"`
	Name       string              `json:"name"`
	Tags       []string            `json:"tags,omitempty"`
	Emails     []ContactValue      `json:"emails,omitempty"`
	Phones     []ContactValue      `json:"phones,omitempty"`
	Avatar     *SourceAvatar       `json:"avatar,omitempty"`
	Accounts   map[string][]string `json:"accounts,omitempty"`
	ETag       string              `json:"etag,omitempty"`
}

type SourceAvatar struct {
	Data   []byte `json:"-"`
	MIME   string `json:"mime,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	URL    string `json:"url,omitempty"`
}

type ImportChange struct {
	Action   string        `json:"action"`
	PersonID string        `json:"person_id,omitempty"`
	Name     string        `json:"name"`
	Source   SourceContact `json:"source"`
	Path     string        `json:"path,omitempty"`
}
