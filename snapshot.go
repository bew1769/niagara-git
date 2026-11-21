package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// snapshot copies a station (or the newest backup) into a git repository and
// commits only when the tracked bytes changed. Keystores, certificates,
// histories, alarms, and logs are left out.

type snapshotOpts struct {
	source string
	repo   string
	shared bool
	push   bool
}

func snapshot(opts snapshotOpts) error {
	if opts.source == "" || opts.repo == "" {
		return fmt.Errorf("niagara-git: snapshot requires --source and --repo\n")
	}
	root, err := ensureRepo(opts.repo)
	if err != nil {
		return err
	}
	// Textconv config is local and harmless to refresh. --store-xml is left as the repo already set it.
	if err := install(root, nil); err != nil {
		return err
	}
	items, err := snapshotItems(opts.source, opts.shared)
	if err != nil {
		return err
	}
	var rels []string
	for _, item := range items {
		if err := writeSnapshotFile(root, item); err != nil {
			return err
		}
		rels = append(rels, item.rel)
	}
	add := append([]string{"add", "--", ".gitignore", ".gitattributes"}, rels...)
	if err := gitRun(root, add...); err != nil {
		return err
	}
	dirty, err := gitCachedDirty(root)
	if err != nil {
		return err
	}
	if !dirty {
		fmt.Fprintf(os.Stderr, "niagara-git: snapshot unchanged\n")
		return nil
	}
	msg := "snapshot " + time.Now().UTC().Format(time.RFC3339)
	if err := gitRun(root, "commit", "-m", msg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "niagara-git: committed %s\n", msg)
	if opts.push {
		if err := gitPush(root); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "niagara-git: pushed\n")
	}
	return nil
}

type snapshotItem struct {
	rel  string
	data []byte
}

func snapshotItems(source string, shared bool) ([]snapshotItem, error) {
	source = expandUser(source)
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("niagara-git: snapshot: %w", err)
	}
	if !info.IsDir() {
		return itemsFromFile(source, shared)
	}
	if _, err := os.Stat(filepath.Join(source, "config.bog")); err == nil {
		return itemsFromStation(source, shared)
	}
	newest, err := newestBackup(source)
	if err != nil {
		return nil, err
	}
	return itemsFromFile(newest, shared)
}

func itemsFromStation(dir string, shared bool) ([]snapshotItem, error) {
	bog, err := os.ReadFile(filepath.Join(dir, "config.bog"))
	if err != nil {
		return nil, err
	}
	items := []snapshotItem{{rel: "config.bog", data: bog}}
	if !shared {
		return items, nil
	}
	sharedRoot := filepath.Join(dir, "shared")
	if _, err := os.Stat(sharedRoot); err != nil {
		if os.IsNotExist(err) {
			return items, nil
		}
		return nil, err
	}
	err = filepath.Walk(sharedRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return fmt.Errorf("niagara-git: snapshot refused path %s", rel)
		}
		kind, _ := classify(rel)
		if kind != kindShow {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		items = append(items, snapshotItem{rel: rel, data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func itemsFromFile(path string, shared bool) ([]snapshotItem, error) {
	lower := strings.ToLower(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.HasSuffix(lower, ".bog"):
		name := filepath.Base(path)
		k, _ := classify(name)
		if k != kindShow {
			return nil, fmt.Errorf("niagara-git: snapshot: refusing %s", name)
		}
		return []snapshotItem{{rel: filepath.ToSlash(name), data: data}}, nil
	case strings.HasSuffix(lower, ".dist"):
		return itemsFromDist(data, shared)
	default:
		return nil, fmt.Errorf("niagara-git: snapshot: %s is not a station directory, .bog, or .dist", path)
	}
}

func itemsFromDist(data []byte, shared bool) ([]snapshotItem, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("niagara-git: snapshot: backup is not a zip: %w", err)
	}
	var bog *zip.File
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(filepath.FromSlash(file.Name))
		if base != "config.bog" {
			continue
		}
		if bog == nil || len(file.Name) < len(bog.Name) {
			bog = file
		}
	}
	if bog == nil {
		return nil, fmt.Errorf("niagara-git: snapshot: backup has no config.bog")
	}
	payload, err := readZipFile(bog)
	if err != nil {
		return nil, err
	}
	items := []snapshotItem{{rel: "config.bog", data: payload}}
	if !shared {
		return items, nil
	}
	prefix := strings.TrimSuffix(bog.Name, "config.bog")
	sharedPrefix := prefix + "shared/"
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasPrefix(file.Name, sharedPrefix) {
			continue
		}
		rel := "shared/" + strings.TrimPrefix(file.Name, sharedPrefix)
		if k, _ := classify(rel); k != kindShow {
			continue
		}
		body, err := readZipFile(file)
		if err != nil {
			return nil, err
		}
		items = append(items, snapshotItem{rel: rel, data: body})
	}
	return items, nil
}

func newestBackup(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var best string
	var bestTime time.Time
	var bestName string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".bog") && !strings.HasSuffix(lower, ".dist") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		if best == "" || info.ModTime().After(bestTime) || (info.ModTime().Equal(bestTime) && name > bestName) {
			best = filepath.Join(dir, name)
			bestTime = info.ModTime()
			bestName = name
		}
	}
	if best == "" {
		return "", fmt.Errorf("niagara-git: snapshot: %s has no .bog or .dist backup", dir)
	}
	return best, nil
}

func writeSnapshotFile(repo string, item snapshotItem) error {
	if k, _ := classify(item.rel); k != kindShow {
		return fmt.Errorf("niagara-git: snapshot refused %s", item.rel)
	}
	dest := filepath.Join(repo, filepath.FromSlash(item.rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, item.data, 0o644)
}

func ensureRepo(path string) (string, error) {
	path = expandUser(path)
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	created := false
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return "", err
		}
		created = true
	} else if !info.IsDir() {
		return "", fmt.Errorf("niagara-git: snapshot: not a directory: %s\n", abs)
	}
	root := canonicalPath(abs)
	if exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree").Run() != nil {
		if err := gitRun(root, "init"); err != nil {
			return "", err
		}
		created = true
	}
	if created {
		// Station zips and the text store must not be rewritten as text.
		// Windows Git often has core.autocrlf=true, which would change a
		// snapshot commit every time a Px or nav file is copied.
		_ = gitRun(root, "config", "core.autocrlf", "false")
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("niagara-git: snapshot: %w", err)
	}
	top := canonicalPath(strings.TrimSpace(string(out)))
	if !sameDir(top, root) {
		return "", fmt.Errorf("niagara-git: snapshot: %s is inside %s; pass the repository root\n", root, top)
	}
	return root, nil
}

// canonicalPath resolves symlinks and 8.3 short names so Git's toplevel
// (often a long path with forward slashes) matches the path Go was given.
func canonicalPath(p string) string {
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

func sameDir(a, b string) bool {
	a = canonicalPath(a)
	b = canonicalPath(b)
	if a == b {
		return true
	}
	// Windows paths are case-insensitive, and Git may spell the drive letter
	// differently from os.Getwd or os.TempDir.
	if runtime.GOOS == "windows" && strings.EqualFold(a, b) {
		return true
	}
	ia, ea := os.Stat(a)
	ib, eb := os.Stat(b)
	return ea == nil && eb == nil && os.SameFile(ia, ib)
}

func gitRun(repo string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text == "" {
			return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), text)
	}
	return nil
}

func gitCachedDirty(repo string) (bool, error) {
	cmd := exec.Command("git", "-C", repo, "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}

func gitPush(repo string) error {
	if exec.Command("git", "-C", repo, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}").Run() != nil {
		return gitRun(repo, "push", "-u", "origin", "HEAD")
	}
	return gitRun(repo, "push")
}
