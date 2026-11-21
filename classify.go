package main

import (
	"path"
	"regexp"
	"strings"
)

// Which members of a station backup are worth diffing.
//
// Niagara 4 layout, from Tridium's AX-to-N4 migration guide and platform guide:
// the protected station home holds config.bog, timestamped config.backup copies,
// and the alarm, history, dataRecovery, provisioningNiagara, and
// niagaraDriver_nVirtual folders. shared/ holds graphics. Certificate stores
// live under the user home security folder (Station Security Guide).

type kind string

const (
	kindShow   kind = "show"
	kindNoisy  kind = "noisy"
	kindSecret kind = "secret"
	kindUnsafe kind = "unsafe"
)

var noisyDirs = map[string]struct{}{
	"alarm": {}, "history": {}, "logs": {}, "logging": {},
	"datarecovery": {}, "provisioningniagara": {}, "niagaradriver_nvirtual": {},
	"__macosx": {},
}

var secretDirs = map[string]struct{}{
	"security": {}, "certmanagement": {}, "certificates": {}, "keystores": {},
}

var secretFilenames = map[string]struct{}{
	"keystore.jceks": {}, "cacerts.jceks": {}, "exemptions.tes": {},
}

var secretExts = map[string]struct{}{
	".jceks": {}, ".bcfks": {}, ".pem": {}, ".key": {}, ".p12": {}, ".pfx": {},
	".jks": {}, ".keystore": {}, ".cer": {}, ".crt": {}, ".der": {}, ".priv": {},
	".tes": {},
}

var noisyFilenames = map[string]struct{}{
	"station.out": {}, "security.out": {}, "audit.out": {},
	".ds_store": {}, "thumbs.db": {},
}

var noisyExts = map[string]struct{}{
	".log": {}, ".hdb": {}, ".adb": {},
}

var configBackup = regexp.MustCompile(`(?i)^config(?:\.backup\.|_backup_).+\.bog$`)
var driveLetter = regexp.MustCompile(`^[A-Za-z]:`)

func fold(s string) string {
	// Station paths are ASCII. strings.ToLower matches Python casefold there.
	return strings.ToLower(s)
}

// classify returns the kind and the display path for a zip member name.
// Secret wins over noisy so a key left under logs/ is still secret.
func classify(name string) (kind, string) {
	unsafe, display := normalizeZipPath(name)
	if unsafe {
		return kindUnsafe, display
	}
	parts := strings.Split(display, "/")
	filename := fold(parts[len(parts)-1])
	var dirs []string
	for _, part := range parts[:len(parts)-1] {
		dirs = append(dirs, fold(part))
	}
	if anyIn(dirs, secretDirs) || inSet(filename, secretFilenames) || inSet(extOf(filename), secretExts) {
		return kindSecret, display
	}
	if anyIn(dirs, noisyDirs) || inSet(filename, noisyFilenames) || inSet(extOf(filename), noisyExts) || configBackup.MatchString(filename) {
		return kindNoisy, display
	}
	return kindShow, display
}

// noisyCategory is the stable bucket for a path already classified as noisy.
func noisyCategory(displayPath string) string {
	parts := strings.Split(displayPath, "/")
	for _, part := range parts[:len(parts)-1] {
		folded := fold(part)
		if _, ok := noisyDirs[folded]; ok {
			return folded
		}
	}
	name := fold(parts[len(parts)-1])
	if configBackup.MatchString(name) {
		return "config.backup"
	}
	if _, ok := noisyFilenames[name]; ok {
		switch name {
		case "station.out", "security.out", "audit.out":
			return "*.out"
		default:
			return name
		}
	}
	if ext := extOf(name); inSet(ext, noisyExts) {
		return "*" + ext
	}
	return "noisy"
}

func normalizeZipPath(name string) (unsafe bool, display string) {
	raw := strings.ReplaceAll(name, "\\", "/")
	absolute := strings.HasPrefix(raw, "/") || driveLetter.MatchString(raw)
	var parts []string
	for _, part := range strings.Split(raw, "/") {
		if part == "" || part == "." {
			continue
		}
		parts = append(parts, part)
	}
	display = strings.Join(parts, "/")
	if display == "" {
		display = raw
	}
	if absolute || len(parts) == 0 || hasDotDot(parts) {
		return true, display
	}
	return false, display
}

func hasDotDot(parts []string) bool {
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}

func anyIn(parts []string, set map[string]struct{}) bool {
	for _, part := range parts {
		if _, ok := set[part]; ok {
			return true
		}
	}
	return false
}

func inSet(s string, set map[string]struct{}) bool {
	_, ok := set[s]
	return ok
}

// extOf matches pathlib.PurePosixPath.suffix: a leading-dot name with no
// further dot has an empty suffix.
func extOf(name string) string {
	base := path.Base(name)
	if strings.HasPrefix(base, ".") && !strings.Contains(base[1:], ".") {
		return ""
	}
	ext := path.Ext(base)
	return fold(ext)
}
