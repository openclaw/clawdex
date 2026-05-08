package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigLoadWriteAndResolve(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := DefaultConfig()
	cfg.RepoPath = filepath.Join(dir, "contacts")
	cfg.Git.Remote = ""
	if err := WriteConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RepoPath != cfg.RepoPath || loaded.Git.Branch != "main" {
		t.Fatalf("loaded = %#v", loaded)
	}
	if got, err := ResolveRepoPath("", loaded); err != nil || got != cfg.RepoPath {
		t.Fatalf("repo path = %q err=%v", got, err)
	}
	t.Setenv(RepoEnv, filepath.Join(dir, "envrepo"))
	if got, _ := ResolveRepoPath("", loaded); !strings.HasSuffix(got, "envrepo") {
		t.Fatalf("env repo path = %q", got)
	}
}

func TestNormalizeFillsDefaults(t *testing.T) {
	cfg := Config{}
	cfg.Normalize()
	if cfg.Version != 1 || cfg.RepoPath == "" || cfg.Git.Remote != DefaultRemote || cfg.Git.Branch != "main" || cfg.Google.Adapter != "gog" {
		t.Fatalf("cfg = %#v", cfg)
	}
	got, err := ResolveRepoPath("/tmp/direct", cfg)
	if err != nil || got != "/tmp/direct" {
		t.Fatalf("direct repo = %q err=%v", got, err)
	}
	t.Setenv(RepoEnv, "")
	if _, err := ResolveRepoPath("", Config{}); err == nil {
		t.Fatal("expected empty repo config error")
	}
}

func TestLoadConfigMissingAndBad(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.toml")
	cfg, err := LoadConfig(missing)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Git.Remote != DefaultRemote {
		t.Fatalf("cfg = %#v", cfg)
	}
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(bad); err == nil {
		t.Fatal("expected bad config error")
	}
	if got := ResolveConfigPath(missing); got != missing {
		t.Fatalf("config path = %q", got)
	}
}

func TestRepoInitRequireAndGit(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.RepoPath = dir
	cfg.Git.Remote = ""
	r := Open(dir, cfg)
	if err := r.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{r.PeopleDir(), r.IndexDir(), r.RepairDir(), filepath.Join(dir, ".git"), filepath.Join(dir, "clawdex.toml")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	if err := r.Require(); err != nil {
		t.Fatal(err)
	}
	if opts := r.MirrorOptions(); opts.RepoPath != dir || opts.Branch != "main" {
		t.Fatalf("opts = %#v", opts)
	}
	dirty, err := r.Dirty(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Fatal("expected dirty repo after init")
	}
	committed, err := r.Commit(t.Context(), "test: init")
	if err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("expected commit")
	}
}

func TestRepoGuardErrors(t *testing.T) {
	cfg := DefaultConfig()
	if err := Open("", cfg).Init(t.Context()); err == nil {
		t.Fatal("expected empty init path error")
	}
	if err := Open("", cfg).Require(); err == nil {
		t.Fatal("expected empty require path error")
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if err := Open(missing, cfg).Require(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("require missing err = %v", err)
	}
}

func TestRequireFailsBeforeInit(t *testing.T) {
	err := Open(t.TempDir(), DefaultConfig()).Require()
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("err = %v", err)
	}
	if got := escapeTOML(`a"b\c`); got != `a\"b\\c` {
		t.Fatalf("escaped = %q", got)
	}
}

func TestRepoInitGuardsAndRemoteLocal(t *testing.T) {
	if err := Open("", DefaultConfig()).Init(t.Context()); err == nil {
		t.Fatal("expected empty path error")
	}
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote.git")
	if err := os.Mkdir(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "init", "--bare")
	cfg := DefaultConfig()
	cfg.RepoPath = filepath.Join(dir, "work")
	cfg.Git.Remote = remote
	r := Open(cfg.RepoPath, cfg)
	if err := r.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoPath, "people", "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if committed, err := r.Commit(t.Context(), "test: remote"); err != nil || !committed {
		t.Fatalf("commit=%v err=%v", committed, err)
	}
	if err := r.Push(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Pull(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
