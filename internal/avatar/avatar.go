package avatar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openclaw/clawdex/internal/model"
)

const DirName = "avatars"

type Problem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func InspectBytes(data []byte) (model.SourceAvatar, error) {
	if len(data) == 0 {
		return model.SourceAvatar{}, errors.New("avatar data is empty")
	}
	mime := sniff(data)
	sum := sha256.Sum256(data)
	return model.SourceAvatar{
		Data:   append([]byte(nil), data...),
		MIME:   mime,
		SHA256: hex.EncodeToString(sum[:]),
	}, nil
}

func InspectFile(path string) (model.AvatarRef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.AvatarRef{}, err
	}
	source, err := InspectBytes(data)
	if err != nil {
		return model.AvatarRef{}, err
	}
	width, height := dimensions(data)
	return model.AvatarRef{
		Path:   path,
		MIME:   source.MIME,
		SHA256: source.SHA256,
		Width:  width,
		Height: height,
	}, nil
}

func SetManual(person model.Person, srcPath string, now time.Time) (model.Person, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return model.Person{}, err
	}
	source, err := InspectBytes(data)
	if err != nil {
		return model.Person{}, err
	}
	ref, err := write(person, source, "manual", now)
	if err != nil {
		return model.Person{}, err
	}
	person.Avatar = ref
	return person, nil
}

func SetImported(person model.Person, source model.SourceAvatar, sourceName string, now time.Time) (model.Person, bool, error) {
	if len(source.Data) == 0 {
		return person, false, nil
	}
	if source.SHA256 == "" || source.MIME == "" {
		var err error
		source, err = InspectBytes(source.Data)
		if err != nil {
			return model.Person{}, false, err
		}
	}
	current := person.Avatar
	if current.SHA256 == source.SHA256 {
		return person, false, nil
	}
	if current.Path != "" && current.Source != "" && current.Source != sourceName {
		return person, false, nil
	}
	ref, err := write(person, source, sourceName, now)
	if err != nil {
		return model.Person{}, false, err
	}
	person.Avatar = ref
	return person, true, nil
}

func Clear(person model.Person) model.Person {
	person.Avatar = model.AvatarRef{}
	return person
}

func Validate(person model.Person) []Problem {
	if strings.TrimSpace(person.Avatar.Path) == "" {
		return nil
	}
	path, err := absolutePath(person, person.Avatar.Path)
	if err != nil {
		return []Problem{{Path: person.Path, Message: err.Error()}}
	}
	ref, err := InspectFile(path)
	if err != nil {
		return []Problem{{Path: path, Message: "avatar file missing or unreadable: " + err.Error()}}
	}
	var problems []Problem
	if person.Avatar.SHA256 != "" && person.Avatar.SHA256 != ref.SHA256 {
		problems = append(problems, Problem{Path: path, Message: "avatar sha256 metadata is stale"})
	}
	if person.Avatar.MIME != "" && person.Avatar.MIME != ref.MIME {
		problems = append(problems, Problem{Path: path, Message: "avatar mime metadata is stale"})
	}
	return problems
}

func RepairMetadata(person model.Person, now time.Time) (model.Person, bool, error) {
	if strings.TrimSpace(person.Avatar.Path) == "" {
		return person, false, nil
	}
	path, err := absolutePath(person, person.Avatar.Path)
	if err != nil {
		return person, false, err
	}
	ref, err := InspectFile(path)
	if err != nil {
		return person, false, err
	}
	ref.Path = person.Avatar.Path
	ref.Source = person.Avatar.Source
	ref.UpdatedAt = person.Avatar.UpdatedAt
	if ref.UpdatedAt.IsZero() {
		ref.UpdatedAt = now.UTC()
	}
	changed := ref.MIME != person.Avatar.MIME || ref.SHA256 != person.Avatar.SHA256 || ref.Width != person.Avatar.Width || ref.Height != person.Avatar.Height || ref.UpdatedAt != person.Avatar.UpdatedAt
	person.Avatar = ref
	return person, changed, nil
}

func AbsolutePath(person model.Person) (string, error) {
	return absolutePath(person, person.Avatar.Path)
}

func write(person model.Person, source model.SourceAvatar, sourceName string, now time.Time) (model.AvatarRef, error) {
	width, height := dimensions(source.Data)
	ext := extension(source.MIME)
	if ext == "" {
		ext = extension(sniff(source.Data))
	}
	if ext == "" {
		ext = ".img"
	}
	rel := filepath.Join(DirName, "avatar"+ext)
	dest, err := absolutePath(person, rel)
	if err != nil {
		return model.AvatarRef{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return model.AvatarRef{}, err
	}
	if err := os.WriteFile(dest, source.Data, 0o600); err != nil {
		return model.AvatarRef{}, err
	}
	mime := strings.TrimSpace(source.MIME)
	if mime == "" {
		mime = sniff(source.Data)
	}
	return model.AvatarRef{
		Path:      filepath.ToSlash(rel),
		Source:    strings.TrimSpace(sourceName),
		MIME:      mime,
		SHA256:    source.SHA256,
		Width:     width,
		Height:    height,
		UpdatedAt: now.UTC(),
	}, nil
}

func absolutePath(person model.Person, rel string) (string, error) {
	if strings.TrimSpace(person.Path) == "" {
		return "", errors.New("person path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("avatar path must be relative: %s", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("avatar path escaped person directory: %s", rel)
	}
	base := filepath.Dir(person.Path)
	return filepath.Join(base, clean), nil
}

func dimensions(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func sniff(data []byte) string {
	mime := http.DetectContentType(data)
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]
	}
	return mime
}

func extension(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
