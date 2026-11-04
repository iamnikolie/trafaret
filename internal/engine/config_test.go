package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- config directory resolution -------------------------------------------

// writeManifest creates dir/manifest.yaml and returns dir.
func writeManifest(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte("projects: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func evalDir(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolveConfigFlagWinsOverEnvAndCwd(t *testing.T) {
	root := evalDir(t, t.TempDir())
	flagDir := writeManifest(t, filepath.Join(root, "flag"))
	envDir := writeManifest(t, filepath.Join(root, "env"))
	cwdDir := writeManifest(t, filepath.Join(root, "cwd"))

	dir, manifest, err := resolveConfig(flagDir, "", envDir, cwdDir)
	if err != nil {
		t.Fatal(err)
	}
	if dir != flagDir || manifest != filepath.Join(flagDir, manifestName) {
		t.Fatalf("--config must win: got %s, %s", dir, manifest)
	}
}

func TestResolveConfigEnvWinsOverCwd(t *testing.T) {
	root := evalDir(t, t.TempDir())
	envDir := writeManifest(t, filepath.Join(root, "env"))
	cwdDir := writeManifest(t, filepath.Join(root, "cwd"))

	dir, _, err := resolveConfig("", "", envDir, cwdDir)
	if err != nil {
		t.Fatal(err)
	}
	if dir != envDir {
		t.Fatalf("$TRAFARET_CONFIG must win over cwd: got %s want %s", dir, envDir)
	}
}

func TestResolveConfigWalksUpFromCwd(t *testing.T) {
	root := evalDir(t, t.TempDir())
	cfg := writeManifest(t, filepath.Join(root, "cfg"))
	deep := filepath.Join(cfg, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, manifest, err := resolveConfig("", "", "", deep)
	if err != nil {
		t.Fatal(err)
	}
	if dir != cfg || manifest != filepath.Join(cfg, manifestName) {
		t.Fatalf("walk-up: got %s, %s", dir, manifest)
	}
}

func TestResolveConfigNearestAncestorWins(t *testing.T) {
	root := evalDir(t, t.TempDir())
	outer := writeManifest(t, filepath.Join(root, "outer"))
	inner := writeManifest(t, filepath.Join(outer, "inner"))
	dir, _, err := resolveConfig("", "", "", filepath.Join(inner, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if dir != inner {
		t.Fatalf("nearest ancestor must win: got %s want %s", dir, inner)
	}
}

func TestResolveConfigNotFound(t *testing.T) {
	// A fresh temp dir has no manifest.yaml on its way up to the filesystem root
	// unless the host has one at "/" or the temp parent — skip then.
	root := evalDir(t, t.TempDir())
	for d := root; ; d = filepath.Dir(d) {
		if isFile(filepath.Join(d, manifestName)) {
			t.Skipf("host has a %s at %s", manifestName, d)
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	_, _, err := resolveConfig("", "", "", root)
	if err == nil || !strings.Contains(err.Error(), "no "+manifestName+" found") || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("want a clear not-found error naming --config, got %v", err)
	}
}

func TestResolveConfigFlagWithoutManifestIsAnError(t *testing.T) {
	empty := evalDir(t, t.TempDir())
	_, _, err := resolveConfig(empty, "", "", empty)
	if err == nil || !strings.Contains(err.Error(), "--config") || !strings.Contains(err.Error(), manifestName) {
		t.Fatalf("want error about missing manifest under --config, got %v", err)
	}
	_, _, err = resolveConfig("", "", empty, empty)
	if err == nil || !strings.Contains(err.Error(), envConfig) {
		t.Fatalf("want error naming $%s, got %v", envConfig, err)
	}
	_, _, err = resolveConfig(filepath.Join(empty, "nope"), "", "", empty)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("want not-a-directory error, got %v", err)
	}
}

func TestResolveConfigManifestFlagSetsConfigDirToItsDir(t *testing.T) {
	root := evalDir(t, t.TempDir())
	dir := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := filepath.Join(dir, "custom-name.yaml")
	if err := os.WriteFile(m, []byte("projects: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := writeManifest(t, filepath.Join(root, "other"))

	gotDir, gotManifest, err := resolveConfig(other, m, other, other)
	if err != nil {
		t.Fatal(err)
	}
	if gotDir != dir || gotManifest != m {
		t.Fatalf("--manifest: config dir must be the manifest's dir; got %s, %s", gotDir, gotManifest)
	}
	if _, _, err := resolveConfig("", filepath.Join(dir, "missing.yaml"), "", root); err == nil {
		t.Fatal("a missing --manifest file must be an error")
	}
}

func TestResolveConfigRelativeAndHomePaths(t *testing.T) {
	root := evalDir(t, t.TempDir())
	cfg := writeManifest(t, filepath.Join(root, "rel"))
	t.Chdir(root)
	dir, _, err := resolveConfig("rel", "", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if evalDir(t, dir) != cfg {
		t.Fatalf("relative --config: got %s want %s", dir, cfg)
	}

	t.Setenv("HOME", root)
	dir, _, err = resolveConfig("~/rel", "", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if evalDir(t, dir) != cfg {
		t.Fatalf("~ in --config: got %s want %s", dir, cfg)
	}
}

func TestNewEngineUsesFlagsEnvAndWalkUp(t *testing.T) {
	root := evalDir(t, t.TempDir())
	cfg := writeManifest(t, filepath.Join(root, "cfg"))
	t.Chdir(root)

	e, err := New(Options{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if e.ConfigDir != cfg || e.GlobalDir() != filepath.Join(cfg, "global") || e.ProjectsDir() != filepath.Join(cfg, "projects") {
		t.Fatalf("engine paths wrong: %+v", e)
	}
	if e.OverlayDir != "" {
		t.Fatalf("no <config>/templates exists, overlay must be empty, got %q", e.OverlayDir)
	}

	t.Setenv(envConfig, cfg)
	e, err = New(Options{})
	if err != nil || e.ConfigDir != cfg {
		t.Fatalf("env: %+v %v", e, err)
	}

	t.Setenv(envConfig, "")
	t.Chdir(filepath.Join(cfg))
	e, err = New(Options{})
	if err != nil || evalDir(t, e.ConfigDir) != cfg {
		t.Fatalf("walk-up: %+v %v", e, err)
	}
}
