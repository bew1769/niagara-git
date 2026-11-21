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

const (
	blockBegin = "# niagara-git:begin"
	blockEnd   = "# niagara-git:end"
)

//go:embed templates/gitattributes
var attrTemplate string

//go:embed templates/gitignore
var ignoreTemplate string

var blockPattern = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(blockBegin) + `.*?` + regexp.QuoteMeta(blockEnd) + `\n?`)

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

func install(repo string) error {
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
	fmt.Fprintf(os.Stderr, "niagara-git: configured %s\n", root)
	fmt.Fprintf(os.Stderr, "niagara-git: commit .gitattributes and .gitignore. Each clone must run the installer; the diff command is local git config.\n")
	return nil
}

// mergeBlock inserts or replaces the marked niagara-git block.
func mergeBlock(existing, block string) string {
	text := strings.ReplaceAll(existing, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	snippet := strings.Trim(block, "\n") + "\n"
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if blockPattern.MatchString(text) {
		replaced := blockPattern.ReplaceAllString(text, "\x00")
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

func gitConfig(repo, key, value string) error {
	return exec.Command("git", "-C", repo, "config", "--local", key, value).Run()
}
