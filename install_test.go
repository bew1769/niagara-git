package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestShellQuoteAndTextconvCommand(t *testing.T) {
	if shellQuote("a b") != "'a b'" {
		t.Fatal(shellQuote("a b"))
	}
	if shellQuote("it's") != `'it'\''s'` {
		t.Fatal(shellQuote("it's"))
	}
	command := textconvCommand("/usr/bin/niagara-git", "bog")
	if !strings.HasSuffix(command, "--format 1 bog") {
		t.Fatal(command)
	}
	if !strings.Contains(textconvCommand("/usr/bin/niagara-git", "dist"), " dist") {
		t.Fatal("dist command")
	}
}

func TestMergeBlockIsIdempotent(t *testing.T) {
	block := "# niagara-git:begin\n*.bog diff=niagara-bog\n# niagara-git:end\n"
	first := mergeBlock("# local\n", block)
	if mergeBlock(first, block) != first {
		t.Fatal("re-merge changed the file")
	}
	if strings.Count(first, "# niagara-git:begin") != 1 {
		t.Fatal(first)
	}
	updated := strings.ReplaceAll(block, "niagara-bog", "niagara-bog-v2")
	third := mergeBlock(first, updated)
	if !strings.Contains(third, "niagara-bog-v2") || strings.Contains(third, "diff=niagara-bog\n") {
		t.Fatal(third)
	}
	if !strings.HasPrefix(third, "# local\n") {
		t.Fatal(third)
	}
}

func TestRepoCopiesContainTemplates(t *testing.T) {
	attrs, err := os.ReadFile(".gitattributes")
	if err != nil {
		t.Fatal(err)
	}
	ignore, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(attrs), strings.TrimSpace(attrTemplate)) {
		t.Fatal(".gitattributes is missing the embedded template")
	}
	if !strings.Contains(string(ignore), strings.TrimSpace(ignoreTemplate)) {
		t.Fatal(".gitignore is missing the embedded template")
	}
}

func TestInstallRegistersDriversAndGitDiff(t *testing.T) {
	bin := testBinary(t)
	repo := t.TempDir()
	git(t, repo, "init")
	git(t, repo, "config", "user.email", "niagara-git@example.com")
	git(t, repo, "config", "user.name", "niagara-git tests")
	git(t, repo, "config", "core.autocrlf", "false")
	git(t, repo, "config", "color.ui", "false")
	runBin(t, bin, "install", repo)

	bogCmd := strings.TrimSpace(git(t, repo, "config", "--local", "--get", "diff.niagara-bog.textconv"))
	distCmd := strings.TrimSpace(git(t, repo, "config", "--local", "--get", "diff.niagara-dist.textconv"))
	if !strings.Contains(bogCmd, "--format 1 bog") {
		t.Fatalf("bog textconv: %s", bogCmd)
	}
	if !strings.Contains(distCmd, "--format 1 dist") {
		t.Fatalf("dist textconv: %s", distCmd)
	}
	if strings.TrimSpace(git(t, repo, "config", "--local", "--get", "diff.niagara-bog.cachetextconv")) != "true" {
		t.Fatal("cachetextconv")
	}

	runBin(t, bin, "install", repo)
	attrs, _ := os.ReadFile(filepath.Join(repo, ".gitattributes"))
	ignore, _ := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if strings.Count(string(attrs), "# niagara-git:begin") != 1 || strings.Count(string(ignore), "# niagara-git:begin") != 1 {
		t.Fatal("template block was duplicated")
	}

	bog := filepath.Join(repo, "config.bog")
	if err := os.WriteFile(bog, mustRead(t, "examples/demoPlant/station/config.bog"), 0o644); err != nil {
		t.Fatal(err)
	}
	pxDir := filepath.Join(repo, "shared", "px")
	if err := os.MkdirAll(pxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pxDir, "AHU1.px"), []byte("<px/>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "file.nav"), []byte("<nav/>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "module.palette"), []byte("<bajaObjectGraph/>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	attr := git(t, repo, "check-attr", "diff", "--", "config.bog", "station.dist", "shared/px/AHU1.px", "file.nav", "module.palette")
	// station.dist does not need to exist for check-attr.
	for _, needle := range []string{
		"config.bog: diff: niagara-bog",
		"shared/px/AHU1.px: diff: set",
		"file.nav: diff: set",
		"module.palette: diff: set",
	} {
		if !strings.Contains(attr, needle) {
			t.Fatalf("missing %s in\n%s", needle, attr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "station.dist"), []byte("PK\x03\x04not-really"), 0o644); err != nil {
		t.Fatal(err)
	}
	attr = git(t, repo, "check-attr", "diff", "--", "station.dist")
	if !strings.Contains(attr, "station.dist: diff: niagara-dist") {
		t.Fatal(attr)
	}

	ignored := []string{
		"history/zone.bin",
		"alarm/alarmData.bin",
		"logs/station.out",
		"security/keystore.jceks",
		"certManagement/client.pem",
		"dataRecovery/state.bin",
		"provisioningNiagara/job.bin",
		"config.backup.20200101.bog",
		"config_backup_200548_0042.bog",
		"trusted/extra.jceks",
	}
	for _, relative := range ignored {
		path := filepath.Join(repo, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !gitIgnored(t, repo, relative) {
			t.Fatalf("expected %s to be ignored", relative)
		}
	}
	for _, relative := range []string{"config.bog", "shared/px/AHU1.px"} {
		if gitIgnored(t, repo, relative) {
			t.Fatalf("expected %s to be tracked", relative)
		}
	}

	git(t, repo, "add", "config.bog", ".gitattributes", ".gitignore")
	git(t, repo, "commit", "-m", "station v1")
	if err := os.WriteFile(bog, mustRead(t, "examples/demoPlant/station-v2/config.bog"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff := gitDiff(t, repo, "config.bog")
	again := gitDiff(t, repo, "config.bog")
	if diff != again {
		t.Fatal("git diff was not deterministic")
	}
	if strings.Contains(diff, "Binary files") {
		t.Fatalf("textconv did not run:\n%s", diff)
	}
	for _, needle := range []string{
		`-          <p n="value" v="72.0"/>`,
		`+          <p n="value" v="70.0"/>`,
		`+      <p h="24" m="control=control" n="Occupied" t="control:BooleanWritable">`,
		`+          <p n="sourceOrd" v="h:24"/>`,
	} {
		if !strings.Contains(diff, needle) {
			t.Fatalf("git diff missing %s\n%s", needle, diff)
		}
	}
}

func TestReadmeExamplesMatchTextconv(t *testing.T) {
	before := convertBog(mustRead(t, "examples/demoPlant/station/config.bog"))
	after := convertBog(mustRead(t, "examples/demoPlant/station-v2/config.bog"))
	diff := unifiedNoIndex(t, before, after)
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	start := strings.Index(text, "<!-- example-diff:start -->")
	end := strings.Index(text, "<!-- example-diff:end -->")
	if start < 0 || end < 0 {
		t.Fatal("readme diff markers missing")
	}
	block := text[start:end]
	for _, line := range strings.Split(block, "\n") {
		if (strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++")) ||
			(strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---")) ||
			strings.HasPrefix(line, " ") ||
			strings.HasPrefix(line, "@@") {
			if !strings.Contains(diff, line) {
				t.Errorf("readme line not in textconv diff:\n%s", line)
			}
		}
	}
	dist := convertDist(mustRead(t, "examples/demoPlant/backups/demoPlant-v1.dist"))
	dstart := strings.Index(text, "<!-- example-dist:start -->")
	dend := strings.Index(text, "<!-- example-dist:end -->")
	for _, line := range strings.Split(text[dstart:dend], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "```") {
			continue
		}
		if !strings.Contains(dist, line) {
			t.Errorf("readme dist line not in textconv output:\n%s", line)
		}
	}
}

var (
	builtBin  string
	buildErr  error
	buildOnce sync.Once
)

func testBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "niagara-git-build")
		if err != nil {
			buildErr = err
			return
		}
		name := "niagara-git"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		builtBin = filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-o", builtBin, ".")
		cmd.Dir = moduleRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("%w\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatalf("build niagara-git: %v", buildErr)
	}
	return builtBin
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func gitDiff(t *testing.T, repo, path string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "diff", "--", path)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff: %v\n%s", err, out)
	}
	return string(out)
}

func gitIgnored(t *testing.T, repo, relative string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "check-ignore", "--", relative)
	err := cmd.Run()
	if err == nil {
		return true
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false
	}
	t.Fatal(err)
	return false
}

func runBin(t *testing.T, bin string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", bin, strings.Join(args, " "), err, out)
	}
}

func unifiedNoIndex(t *testing.T, before, after string) string {
	t.Helper()
	dir := t.TempDir()
	a := filepath.Join(dir, "before")
	b := filepath.Join(dir, "after")
	if err := os.WriteFile(a, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-c", "core.autocrlf=false", "-c", "color.ui=false", "diff", "--no-index", "--unified=3", "--", a, b)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("git diff --no-index: %v\n%s", err, out)
		}
	}
	return string(out)
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
