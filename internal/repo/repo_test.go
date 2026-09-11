package repo

import (
	"errors"
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
	readOnlyDirty, err := r.DirtyReadOnly(t.Context())
	if err != nil || !readOnlyDirty {
		t.Fatalf("read-only dirty=%v err=%v", readOnlyDirty, err)
	}
	if err := r.ValidateRemote(t.Context()); !errors.Is(err, ErrRemoteNotConfigured) {
		t.Fatalf("validate remote err = %v", err)
	}
	committed, err := r.Commit(t.Context(), "test: init")
	if err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("expected commit")
	}
	readOnlyDirty, err = r.DirtyReadOnly(t.Context())
	if err != nil || readOnlyDirty {
		t.Fatalf("clean read-only dirty=%v err=%v", readOnlyDirty, err)
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
	if err := r.ValidateRemote(t.Context()); err != nil {
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
	if _, err := Open(filepath.Join(dir, "missing"), cfg).DirtyReadOnly(t.Context()); err == nil {
		t.Fatal("expected missing repo status error")
	}
}

func TestPullPreservesLocalHistory(t *testing.T) {
	for _, configuredRemote := range []bool{true, false} {
		for _, scenario := range []string{"ahead", "diverged", "fast-forward"} {
			name := scenario + "/origin-only"
			if configuredRemote {
				name = scenario + "/configured-remote"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				remote := filepath.Join(dir, "remote.git")
				runGit(t, dir, "init", "--bare", "--initial-branch=contacts", remote)
				cfg := DefaultConfig()
				cfg.RepoPath = filepath.Join(dir, "local")
				cfg.Git.Remote = remote
				cfg.Git.Branch = "contacts"
				r := Open(cfg.RepoPath, cfg)
				if err := r.Init(t.Context()); err != nil {
					t.Fatal(err)
				}
				if committed, err := r.Commit(t.Context(), "test: initial contacts"); err != nil || !committed {
					t.Fatalf("initial commit=%v err=%v", committed, err)
				}
				if err := r.Push(t.Context()); err != nil {
					t.Fatal(err)
				}
				writer := filepath.Join(dir, "writer")
				runGit(t, dir, "clone", remote, writer)
				if scenario != "fast-forward" {
					if err := os.WriteFile(filepath.Join(r.Path, "local-note.md"), []byte("unpublished note\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					if committed, err := r.Commit(t.Context(), "test: unpublished note"); err != nil || !committed {
						t.Fatalf("local commit=%v err=%v", committed, err)
					}
				}
				before := runGit(t, r.Path, "rev-parse", "HEAD")
				if scenario != "ahead" {
					if err := os.WriteFile(filepath.Join(writer, "remote-note.md"), []byte("remote note\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					other := Open(writer, cfg)
					if committed, err := other.Commit(t.Context(), "test: remote note"); err != nil || !committed {
						t.Fatalf("remote commit=%v err=%v", committed, err)
					}
					runGit(t, writer, "push", "origin", "contacts")
				}
				if configuredRemote {
					// The configured URL must still replace a stale origin before pulling.
					runGit(t, r.Path, "remote", "set-url", "origin", filepath.Join(dir, "stale.git"))
				} else {
					r.Config.Git.Remote = ""
				}
				err := r.Pull(t.Context())
				if (err != nil) != (scenario == "diverged") {
					t.Fatalf("pull %s: %v", scenario, err)
				}
				wantHead := before
				if scenario == "fast-forward" {
					wantHead = runGit(t, writer, "rev-parse", "HEAD")
				} else {
					data, err := os.ReadFile(filepath.Join(r.Path, "local-note.md"))
					if err != nil || string(data) != "unpublished note\n" {
						t.Fatalf("local note lost: %q err=%v", data, err)
					}
				}
				if got := runGit(t, r.Path, "rev-parse", "HEAD"); got != wantHead {
					t.Fatalf("HEAD changed unexpectedly: got %s want %s", got, wantHead)
				}
			})
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
