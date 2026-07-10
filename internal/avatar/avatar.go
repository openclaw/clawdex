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
	"path/filepath"
	"strings"
	"time"

	"github.com/openclaw/clawdex/internal/model"
	"github.com/openclaw/clawdex/internal/safefile"
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
	data, err := safefile.ReadPath(path)
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

func SetManual(root string, person model.Person, srcPath string, now time.Time) (model.Person, error) {
	data, err := safefile.ReadPath(srcPath)
	if err != nil {
		return model.Person{}, err
	}
	source, err := InspectBytes(data)
	if err != nil {
		return model.Person{}, err
	}
	ref, err := write(root, person, source, "manual", now)
	if err != nil {
		return model.Person{}, err
	}
	person.Avatar = ref
	return person, nil
}

// ValidateManual applies SetManual's source and destination checks without
// creating or replacing avatar files.
func ValidateManual(root string, person model.Person, srcPath string) (model.AvatarRef, error) {
	data, err := safefile.ReadPath(srcPath)
	if err != nil {
		return model.AvatarRef{}, err
	}
	source, err := InspectBytes(data)
	if err != nil {
		return model.AvatarRef{}, err
	}
	ref, dest, err := plannedAvatar(root, person, source, "manual", time.Time{})
	if err != nil {
		return model.AvatarRef{}, err
	}
	if err := safefile.ValidateAtomicWrite(root, dest); err != nil {
		return model.AvatarRef{}, err
	}
	return ref, nil
}

func SetImported(root string, person model.Person, source model.SourceAvatar, sourceName string, now time.Time) (model.Person, bool, error) {
	source, changed, err := prepareImported(person, source, sourceName)
	if err != nil || !changed {
		return person, changed, err
	}
	ref, err := write(root, person, source, sourceName, now)
	if err != nil {
		return model.Person{}, false, err
	}
	person.Avatar = ref
	return person, true, nil
}

// ValidateImported applies SetImported's overwrite and destination checks
// without writing. The bool reports whether SetImported would replace bytes.
func ValidateImported(root string, person model.Person, source model.SourceAvatar, sourceName string) (bool, error) {
	source, changed, err := prepareImported(person, source, sourceName)
	if err != nil || !changed {
		return changed, err
	}
	_, dest, err := plannedAvatar(root, person, source, sourceName, time.Time{})
	if err != nil {
		return false, err
	}
	if err := safefile.ValidateAtomicWrite(root, dest); err != nil {
		return false, err
	}
	return true, nil
}

func prepareImported(person model.Person, source model.SourceAvatar, sourceName string) (model.SourceAvatar, bool, error) {
	if len(source.Data) == 0 {
		return source, false, nil
	}
	if source.SHA256 == "" || source.MIME == "" {
		var err error
		source, err = InspectBytes(source.Data)
		if err != nil {
			return model.SourceAvatar{}, false, err
		}
	}
	current := person.Avatar
	if current.SHA256 == source.SHA256 {
		return source, false, nil
	}
	if current.Path != "" && current.Source != "" && current.Source != sourceName {
		return source, false, nil
	}
	return source, true, nil
}

func Clear(person model.Person) model.Person {
	person.Avatar = model.AvatarRef{}
	return person
}

func Validate(root string, person model.Person) []Problem {
	if strings.TrimSpace(person.Avatar.Path) == "" {
		return nil
	}
	relative, err := relativePath(root, person, person.Avatar.Path)
	if err != nil {
		return []Problem{{Path: person.Path, Message: err.Error()}}
	}
	ref, err := inspectRootedFile(root, relative)
	if err != nil {
		return []Problem{{Path: person.Path, Message: "avatar file missing or unreadable: " + err.Error()}}
	}
	path := filepath.Join(root, relative)
	var problems []Problem
	if person.Avatar.SHA256 != "" && person.Avatar.SHA256 != ref.SHA256 {
		problems = append(problems, Problem{Path: path, Message: "avatar sha256 metadata is stale"})
	}
	if person.Avatar.MIME != "" && person.Avatar.MIME != ref.MIME {
		problems = append(problems, Problem{Path: path, Message: "avatar mime metadata is stale"})
	}
	return problems
}

func RepairMetadata(root string, person model.Person, now time.Time) (model.Person, bool, error) {
	if strings.TrimSpace(person.Avatar.Path) == "" {
		return person, false, nil
	}
	relative, err := relativePath(root, person, person.Avatar.Path)
	if err != nil {
		return person, false, err
	}
	ref, err := inspectRootedFile(root, relative)
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

func inspectRootedFile(root, relative string) (model.AvatarRef, error) {
	data, err := safefile.ReadFile(root, relative)
	if err != nil {
		return model.AvatarRef{}, err
	}
	source, err := InspectBytes(data)
	if err != nil {
		return model.AvatarRef{}, err
	}
	width, height := dimensions(data)
	return model.AvatarRef{
		Path:   filepath.Join(root, relative),
		MIME:   source.MIME,
		SHA256: source.SHA256,
		Width:  width,
		Height: height,
	}, nil
}

func AbsolutePath(root string, person model.Person) (string, error) {
	return absolutePath(root, person, person.Avatar.Path)
}

func write(root string, person model.Person, source model.SourceAvatar, sourceName string, now time.Time) (model.AvatarRef, error) {
	ref, dest, err := plannedAvatar(root, person, source, sourceName, now)
	if err != nil {
		return model.AvatarRef{}, err
	}
	if err := safefile.AtomicWriteFile(root, dest, source.Data, 0o600); err != nil {
		return model.AvatarRef{}, err
	}
	return ref, nil
}

func plannedAvatar(root string, person model.Person, source model.SourceAvatar, sourceName string, now time.Time) (model.AvatarRef, string, error) {
	width, height := dimensions(source.Data)
	ext := extension(source.MIME)
	if ext == "" {
		ext = extension(sniff(source.Data))
	}
	if ext == "" {
		ext = ".img"
	}
	rel := filepath.Join(DirName, "avatar"+ext)
	dest, err := relativePath(root, person, rel)
	if err != nil {
		return model.AvatarRef{}, "", err
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
	}, dest, nil
}

func absolutePath(root string, person model.Person, rel string) (string, error) {
	path, err := relativePath(root, person, rel)
	if err != nil {
		return "", err
	}
	return safefile.ExistingPath(root, path)
}

func relativePath(root string, person model.Person, rel string) (string, error) {
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
	return safefile.Relative(root, filepath.Join(base, clean))
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
