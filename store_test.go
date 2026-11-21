package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanSmudgePreservesDemoBog(t *testing.T) {
	original := mustRead(t, "examples/demoPlant/station/config.bog")
	stored, err := clean(original)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(stored, []byte(storeMagic)) {
		t.Fatalf("clean output is not a store")
	}
	again, err := clean(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, again) {
		t.Fatal("clean is not idempotent")
	}
	rebuilt, err := smudge(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err := entriesEqual(original, rebuilt); err != nil {
		t.Fatal(err)
	}
	rebuiltAgain, err := smudge(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rebuilt, rebuiltAgain) {
		t.Fatal("smudge is not deterministic")
	}
	round, err := clean(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, round) {
		t.Fatal("clean(smudge(clean(bog))) changed the blob")
	}
	if convertBog(stored) != convertBog(original) {
		t.Fatal("textconv of the store blob differs from the zip")
	}
}

func TestCleanPreservesOrderNamesAndBinaryEntries(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?><root><p n="A" v="1"/></root>`)
	binary := []byte{0x00, 0x89, 0xff, 'P', 'N', 'G'}
	original := zipWithComment(t, "station comment", []zipEntry{
		{"notes/readme.txt", []byte("keep me first")},
		{"dir/", nil},
		{"file.xml", xml},
		{"images/mark.bin", binary},
	})
	stored, err := clean(original)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, binary) {
		t.Fatal("binary entry was stored raw")
	}
	rebuilt, err := smudge(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err := entriesEqual(original, rebuilt); err != nil {
		t.Fatal(err)
	}
	comment, entries, err := readZipEntries(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if comment != "station comment" {
		t.Fatal(comment)
	}
	if entries[0].name != "notes/readme.txt" || entries[2].name != "file.xml" || entries[3].name != "images/mark.bin" {
		t.Fatalf("order changed: %v", namesOf(entries))
	}
	other := zipWithComment(t, "station comment", []zipEntry{
		{"file.xml", xml},
		{"notes/readme.txt", []byte("keep me first")},
		{"dir/", nil},
		{"images/mark.bin", binary},
	})
	otherStored, err := clean(other)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(stored, otherStored) {
		t.Fatal("entry order was discarded")
	}
}

func TestZipMetadataDoesNotChangeCleanOutput(t *testing.T) {
	xml := []byte(`<root><p n="A"/></root>`)
	first := buildZipAt(t, []zipEntry{{"file.xml", xml}}, time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC))
	second := buildZipAt(t, []zipEntry{{"file.xml", xml}}, time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC))
	left, err := clean(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := clean(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left, right) {
		t.Fatal("zip timestamps leaked into the stored form")
	}
}

func TestGitFilterRoundTrip(t *testing.T) {
	bin := testBinary(t)
	repo := t.TempDir()
	git(t, repo, "init")
	git(t, repo, "config", "user.email", "niagara-git@example.com")
	git(t, repo, "config", "user.name", "niagara-git tests")
	git(t, repo, "config", "commit.gpgsign", "false")
	git(t, repo, "config", "core.autocrlf", "false")
	runBin(t, bin, "install", "--store-xml", repo)

	attr := git(t, repo, "check-attr", "filter", "diff", "text", "--", "config.bog")
	if !strings.Contains(attr, "config.bog: filter: niagara-store") {
		t.Fatal(attr)
	}
	if !strings.Contains(attr, "config.bog: diff: niagara-bog") {
		t.Fatal(attr)
	}

	original := mustRead(t, "examples/demoPlant/station/config.bog")
	if err := os.WriteFile(filepath.Join(repo, "config.bog"), original, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "config.bog", ".gitattributes", ".gitignore")
	git(t, repo, "commit", "-m", "import")
	blob := git(t, repo, "cat-file", "-p", "HEAD:config.bog")
	if !strings.HasPrefix(blob, storeMagic) {
		t.Fatalf("blob was not stored as text: %.80q", blob)
	}
	if strings.TrimSpace(git(t, repo, "status", "--porcelain")) != "" {
		t.Fatalf("spurious status after commit: %s", git(t, repo, "status", "--porcelain"))
	}

	if err := os.Remove(filepath.Join(repo, "config.bog")); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "checkout", "--", "config.bog")
	checked := mustRead(t, filepath.Join(repo, "config.bog"))
	if !bytes.HasPrefix(checked, []byte("PK")) {
		t.Fatal("checkout did not rebuild a zip")
	}
	if err := entriesEqual(original, checked); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(git(t, repo, "status", "--porcelain")) != "" {
		t.Fatalf("checkout left the worktree dirty: %s", git(t, repo, "status", "--porcelain"))
	}

	v2 := mustRead(t, "examples/demoPlant/station-v2/config.bog")
	if err := os.WriteFile(filepath.Join(repo, "config.bog"), v2, 0o644); err != nil {
		t.Fatal(err)
	}
	diff := gitDiff(t, repo, "config.bog")
	if strings.Contains(diff, "Binary files") {
		t.Fatalf("textconv did not run on the stored blob:\n%s", diff)
	}
	if !strings.Contains(diff, `v="70.0"`) {
		t.Fatalf("diff missing setpoint change:\n%s", diff)
	}
}

func TestSmudgePassesZipThrough(t *testing.T) {
	original := mustRead(t, "examples/demoPlant/station/config.bog")
	out, err := smudge(original)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, out) {
		t.Fatal("zip smudge changed the bytes")
	}
}

func namesOf(entries []storedEntry) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.name
	}
	return names
}

func zipWithComment(t *testing.T, comment string, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := zw.SetComment(comment); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate, Modified: storeFixedTime}
		if strings.HasSuffix(entry.name, "/") {
			header.Method = zip.Store
		}
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if entry.data != nil {
			if _, err := w.Write(entry.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
