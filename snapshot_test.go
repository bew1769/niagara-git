package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotStationSkipsSecretsAndRepeats(t *testing.T) {
	bin := testBinary(t)
	station := t.TempDir()
	copyFile(t, "examples/demoPlant/station/config.bog", filepath.Join(station, "config.bog"))
	copyFile(t, "examples/demoPlant/station/shared/px/AHU1.px", filepath.Join(station, "shared", "px", "AHU1.px"))
	secret := filepath.Join(station, "shared", "security", "keystore.jceks")
	if err := os.MkdirAll(filepath.Dir(secret), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("synthetic-keystore-not-a-real-jceks"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(station, "history"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(station, "history", "trend.bin"), []byte("synthetic-history"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := filepath.Join(t.TempDir(), "repo")
	out := runSnapshot(t, bin, "--source", station, "--repo", repo, "--shared")
	if !strings.Contains(out, "committed snapshot ") {
		t.Fatalf("first snapshot: %s", out)
	}
	files := git(t, repo, "ls-files")
	for _, name := range []string{"config.bog", "shared/px/AHU1.px", ".gitignore"} {
		if !strings.Contains(files, name) {
			t.Fatalf("missing %s in\n%s", name, files)
		}
	}
	for _, banned := range []string{"keystore.jceks", "history/", "trend.bin"} {
		if strings.Contains(files, banned) {
			t.Fatalf("committed %s\n%s", banned, files)
		}
	}
	if !strings.Contains(convertBog(mustRead(t, filepath.Join(repo, "config.bog"))), `v="72.0"`) {
		t.Fatal("committed bog is not the demo station")
	}

	again := runSnapshot(t, bin, "--source", station, "--repo", repo, "--shared")
	if !strings.Contains(again, "snapshot unchanged") {
		t.Fatalf("second snapshot: %s", again)
	}
	if strings.TrimSpace(git(t, repo, "rev-list", "--count", "HEAD")) != "1" {
		t.Fatal("unchanged snapshot created a commit")
	}
}

func TestSnapshotBackupFolderPicksNewestAndSkipsSecrets(t *testing.T) {
	bin := testBinary(t)
	backups := t.TempDir()
	v1 := filepath.Join(backups, "demoPlant-v1.dist")
	v2 := filepath.Join(backups, "demoPlant-v2.dist")
	copyFile(t, "examples/demoPlant/backups/demoPlant-v1.dist", v1)
	copyFile(t, "examples/demoPlant/backups/demoPlant-v2.dist", v2)
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	if err := os.Chtimes(v2, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(v1, newer, newer); err != nil {
		t.Fatal(err)
	}

	repo := filepath.Join(t.TempDir(), "repo")
	out := runSnapshot(t, bin, "--source", backups, "--repo", repo, "--shared")
	if !strings.Contains(out, "committed snapshot ") {
		t.Fatalf("snapshot: %s", out)
	}
	bog := convertBog(mustRead(t, filepath.Join(repo, "config.bog")))
	if !strings.Contains(bog, `v="72.0"`) || strings.Contains(bog, `n="Occupied"`) {
		t.Fatal("newest backup was not v1")
	}
	files := git(t, repo, "ls-files")
	if !strings.Contains(files, "shared/px/AHU1.px") {
		t.Fatalf("shared px missing:\n%s", files)
	}
	for _, banned := range []string{".dist", "keystore.jceks", "history/", "alarm/", ".pem", "station.out"} {
		if strings.Contains(files, banned) {
			t.Fatalf("committed %s from the dist\n%s", banned, files)
		}
	}
}

func TestSnapshotPushToLocalRemote(t *testing.T) {
	bin := testBinary(t)
	bare := t.TempDir()
	git(t, bare, "init", "--bare")
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init")
	git(t, repo, "remote", "add", "origin", bare)
	station := t.TempDir()
	copyFile(t, "examples/demoPlant/station/config.bog", filepath.Join(station, "config.bog"))
	out := runSnapshot(t, bin, "--source", station, "--repo", repo, "--push")
	if !strings.Contains(out, "pushed") {
		t.Fatalf("push: %s", out)
	}
	log := git(t, bare, "log", "--oneline")
	if !strings.Contains(log, "snapshot ") {
		t.Fatal(log)
	}
}

func TestEnsureRepoAcceptsAliasOfRoot(t *testing.T) {
	real := t.TempDir()
	git(t, real, "init")
	parent := t.TempDir()
	link := filepath.Join(parent, "repo-link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	got, err := ensureRepo(link)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDir(got, real) {
		t.Fatalf("ensureRepo(%s) = %s, want %s", link, got, real)
	}
	nested := filepath.Join(real, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureRepo(nested); err == nil {
		t.Fatal("subdirectory of a repo was accepted as the repository root")
	}
}

func copyFile(t *testing.T, src, dest string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, mustRead(t, src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runSnapshot(t *testing.T, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"snapshot"}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=niagara-git tests",
		"GIT_AUTHOR_EMAIL=niagara-git@example.com",
		"GIT_COMMITTER_NAME=niagara-git tests",
		"GIT_COMMITTER_EMAIL=niagara-git@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("snapshot: %v\n%s", err, out)
	}
	return string(out)
}
