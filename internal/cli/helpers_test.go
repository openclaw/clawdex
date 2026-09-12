package cli

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func testPaths(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "config.toml"), filepath.Join(dir, "contacts")
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%s:%o", info.Mode().Type(), info.Mode().Perm())
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + target
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%x", sha256.Sum256(data))
		}
		out[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func writeFakeGog(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	name := "gog"
	if runtime.GOOS == "windows" {
		name = "gog.bat"
	}
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%s\\n' '" + output + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeGogExit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gog")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho bad >&2\nexit 4\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeSQLite(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sqlite3")
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat <<'JSON'\n"+output+"\nJSON\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeContactCrawler(t *testing.T, name, contacts string) string {
	t.Helper()
	manifest := `{"schema_version":"crawlkit.control.v1","id":"` + name + `","display_name":"Fake Crawler","binary":{"name":"` + name + `"},"commands":{"contact-export":{"argv":["` + name + `","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`
	return writeFakeContactCrawlerManifest(t, name, manifest, contacts)
}

func writeFakeContactCrawlerManifest(t *testing.T, name, manifest, contacts string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"metadata\" ]; then\n" +
		"cat <<'JSON'\n" + manifest + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"contacts\" ] && [ \"$3\" = \"export\" ]; then\n" +
		"cat <<'JSON'\n" + contacts + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"echo unexpected args: \"$@\" >&2\nexit 2\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func runShell(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
