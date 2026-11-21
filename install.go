package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed templates/gitattributes
var attrTemplate string

//go:embed templates/gitignore
var ignoreTemplate string

//go:embed templates/gitattributes-store
var storeAttrTemplate string

const (
	blockBegin      = "# niagara-git:begin"
	blockEnd        = "# niagara-git:end"
	storeBlockBegin = "# niagara-git-store:begin"
	storeBlockEnd   = "# niagara-git-store:end"
)

func mergeMarked(existing, begin, end, block string) string {
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(begin) + `.*?` + regexp.QuoteMeta(end) + `\n?`)
	text := strings.ReplaceAll(existing, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	snippet := strings.Trim(block, "\n") + "\n"
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if pattern.MatchString(text) {
		replaced := pattern.ReplaceAllString(text, "\x00")
		replaced = strings.Replace(replaced, "\x00", snippet, 1)
		replaced = strings.ReplaceAll(replaced, "\x00", "")
		return replaced
	}
	if text == "" {
		return snippet
	}
	if !strings.HasSuffix(text, "\n\n") {
		return text + "\n" + snippet
	}
	return text + snippet
}

func removeMarked(existing, begin, end string) string {
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(begin) + `.*?` + regexp.QuoteMeta(end) + `\n?`)
	text := strings.ReplaceAll(existing, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = pattern.ReplaceAllString(text, "")
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text
}

// shellQuote uses POSIX single quotes. Git runs textconv through a shell,
// including Git for Windows.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func textconvCommand(exe, mode string) string {
	return shellQuote(filepath.ToSlash(exe)) + " --format " + itoa(formatVersion) + " " + mode
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}
	return filepath.Clean(exe), nil
}

func install(repo string, storeXML *bool) error {
	root, err := resolveRepo(repo)
	if err != nil {
		return err
	}
	if err := mergeTemplate(filepath.Join(root, ".gitattributes"), attrTemplate); err != nil {
		return err
	}
	if err := mergeTemplate(filepath.Join(root, ".gitignore"), ignoreTemplate); err != nil {
		return err
	}
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	if err := gitConfig(root, "diff.niagara-bog.textconv", textconvCommand(exe, "bog")); err != nil {
		return err
	}
	if err := gitConfig(root, "diff.niagara-bog.cachetextconv", "true"); err != nil {
		return err
	}
	if err := gitConfig(root, "diff.niagara-dist.textconv", textconvCommand(exe, "dist")); err != nil {
		return err
	}
	if err := gitConfig(root, "diff.niagara-dist.cachetextconv", "true"); err != nil {
		return err
	}
	switch {
	case storeXML != nil && *storeXML:
		if err := mergeMarkedFile(filepath.Join(root, ".gitattributes"), storeBlockBegin, storeBlockEnd, storeAttrTemplate); err != nil {
			return err
		}
		if err := gitConfig(root, "filter.niagara-store.clean", filterCommand(exe, "clean")); err != nil {
			return err
		}
		if err := gitConfig(root, "filter.niagara-store.smudge", filterCommand(exe, "smudge")); err != nil {
			return err
		}
		if err := gitConfig(root, "filter.niagara-store.required", "true"); err != nil {
			return err
		}
	case storeXML != nil && !*storeXML:
		if err := removeMarkedFile(filepath.Join(root, ".gitattributes"), storeBlockBegin, storeBlockEnd); err != nil {
			return err
		}
		gitConfigUnset(root, "filter.niagara-store.clean")
		gitConfigUnset(root, "filter.niagara-store.smudge")
		gitConfigUnset(root, "filter.niagara-store.required")
	}
	fmt.Fprintf(os.Stderr, "niagara-git: configured %s\n", root)
	fmt.Fprintf(os.Stderr, "niagara-git: commit .gitattributes and .gitignore. Each clone must run the installer; the diff command is local git config.\n")
	if storeXML != nil && *storeXML {
		fmt.Fprintf(os.Stderr, "niagara-git: .bog blobs will be stored as text and rebuilt as zip files on checkout.\n")
	}
	return nil
}

func filterCommand(exe, mode string) string {
	return shellQuote(filepath.ToSlash(exe)) + " " + mode
}

// mergeBlock inserts or replaces the marked niagara-git block.
func mergeBlock(existing, block string) string {
	return mergeMarked(existing, blockBegin, blockEnd, block)
}

func mergeTemplate(dest, block string) error {
	current := ""
	if data, err := os.ReadFile(dest); err == nil {
		current = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(dest, []byte(mergeBlock(current, block)), 0o644)
}

func resolveRepo(repo string) (string, error) {
	if repo == "" {
		out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return "", fmt.Errorf("niagara-git: not a git repository. Pass the station repo path, or run git init first.\n")
		}
		return strings.TrimSpace(string(out)), nil
	}
	repo = expandUser(repo)
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("niagara-git: not a directory: %s\n", abs)
	}
	out, err := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("niagara-git: %s is not a git repository. Run git init there first.\n", abs)
	}
	return strings.TrimSpace(string(out)), nil
}

func expandUser(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func mergeMarkedFile(dest, begin, end, block string) error {
	current := ""
	if data, err := os.ReadFile(dest); err == nil {
		current = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(dest, []byte(mergeMarked(current, begin, end, block)), 0o644)
}

func removeMarkedFile(dest, begin, end string) error {
	data, err := os.ReadFile(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(dest, []byte(removeMarked(string(data), begin, end)), 0o644)
}

func gitConfig(repo, key, value string) error {
	return exec.Command("git", "-C", repo, "config", "--local", key, value).Run()
}

func gitConfigUnset(repo, key string) {
	_ = exec.Command("git", "-C", repo, "config", "--unset", key).Run()
}
