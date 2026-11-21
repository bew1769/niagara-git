package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Textconv views for .bog and .dist files.
//
// Output depends only on archive bytes. Git passes textconv a temporary
// checkout path, so that path is never included.

const (
	formatVersion = 1
	maxEntryBytes = 256 * 1024 * 1024
)

var (
	errEncrypted = errors.New("encrypted")
	errTooLarge  = errors.New("too large")
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func convertBog(data []byte) string {
	if isStore(data) {
		zipped, err := smudge(data)
		if err != nil {
			return opaque("bog", data, "store format could not be rebuilt")
		}
		return convertBogZip(zipped)
	}
	if bytes.HasPrefix(data, []byte("PK")) {
		return convertBogZip(data)
	}
	if looksLikeXML(data) {
		return renderXML(data)
	}
	return opaque("bog", data, "not a zip archive or XML document; passphrase-protected bog files are encrypted")
}

func convertDist(data []byte) string {
	if bytes.HasPrefix(data, []byte("PK")) {
		return convertDistZip(data)
	}
	if looksLikeXML(data) {
		return renderXML(data)
	}
	return opaque("dist", data, "not a zip archive; passphrase-protected distribution files are encrypted")
}

func convertBogZip(data []byte) string {
	members, err := openZip(data)
	if err != nil {
		return opaque("bog", data, "not a readable zip archive: File is not a zip file")
	}
	return renderMembers(members)
}

func convertDistZip(data []byte) string {
	members, err := openZip(data)
	if err != nil {
		return opaque("dist", data, "not a readable zip archive: File is not a zip file")
	}
	return renderDist(members)
}

type zipMember struct {
	name string
	file *zip.File
}

func openZip(data []byte) ([]zipMember, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var members []zipMember
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "/") || file.FileInfo().IsDir() {
			continue
		}
		members = append(members, zipMember{name: file.Name, file: file})
	}
	return members, nil
}

func (m zipMember) declaredSize() uint64 {
	return m.file.UncompressedSize64
}

func (m zipMember) read() ([]byte, error) {
	if m.file.Flags&1 != 0 {
		return nil, errEncrypted
	}
	if m.declaredSize() > maxEntryBytes {
		return nil, errTooLarge
	}
	rc, err := m.file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	payload, err := io.ReadAll(io.LimitReader(rc, maxEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(payload)) > maxEntryBytes {
		return nil, errTooLarge
	}
	return payload, nil
}

func renderDist(members []zipMember) string {
	type shownFile struct {
		path   string
		size   int
		digest string
	}
	type sizedPath struct {
		path string
		size uint64
	}
	type otherFile struct {
		reason string
		path   string
		size   uint64
	}
	var shown []shownFile
	var secrets []sizedPath
	var others []otherFile
	noisy := map[string][2]uint64{}
	type bogDoc struct {
		path string
		xml  string
	}
	var bogs []bogDoc

	ordered := append([]zipMember(nil), members...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return sortKeyLess(ordered[i].name, ordered[j].name)
	})
	for _, member := range ordered {
		k, display := classify(member.name)
		switch k {
		case kindNoisy:
			cat := noisyCategory(display)
			cur := noisy[cat]
			cur[0]++
			cur[1] += member.declaredSize()
			noisy[cat] = cur
			continue
		case kindSecret:
			secrets = append(secrets, sizedPath{path: display, size: member.declaredSize()})
			continue
		case kindUnsafe:
			others = append(others, otherFile{reason: "unsafe", path: display, size: member.declaredSize()})
			continue
		}
		if member.declaredSize() > maxEntryBytes {
			others = append(others, otherFile{reason: "too-large", path: display, size: member.declaredSize()})
			continue
		}
		payload, err := member.read()
		if err != nil {
			others = append(others, otherFile{
				reason: "unreadable (" + errClass(err) + ")",
				path:   display,
				size:   member.declaredSize(),
			})
			continue
		}
		shown = append(shown, shownFile{path: display, size: len(payload), digest: sha256Hex(payload)})
		if strings.HasSuffix(fold(display), ".bog") {
			bogs = append(bogs, bogDoc{path: display, xml: strings.TrimRight(convertBog(payload), "\n")})
		}
	}

	lines := []string{
		fmt.Sprintf("# niagara-git dist format %d", formatVersion),
		fmt.Sprintf("# files %d", len(shown)),
		fmt.Sprintf("# skipped-noisy %d", noisyCount(noisy)),
		fmt.Sprintf("# skipped-secret %d", len(secrets)),
	}
	if len(others) > 0 {
		lines = append(lines, fmt.Sprintf("# skipped-other %d", len(others)))
	}
	lines = append(lines, "", "## files")
	sort.SliceStable(shown, func(i, j int) bool { return pathLess(shown[i].path, shown[j].path) })
	for _, file := range shown {
		lines = append(lines, fmt.Sprintf("%d %s %s", file.size, file.digest, file.path))
	}
	if len(noisy) > 0 {
		lines = append(lines, "", "## skipped-noisy")
		cats := make([]string, 0, len(noisy))
		for cat := range noisy {
			cats = append(cats, cat)
		}
		sort.Strings(cats)
		for _, cat := range cats {
			cur := noisy[cat]
			lines = append(lines, fmt.Sprintf("%s count=%d size=%d", cat, cur[0], cur[1]))
		}
	}
	if len(secrets) > 0 {
		lines = append(lines, "", "## skipped-secret")
		sort.SliceStable(secrets, func(i, j int) bool { return pathLess(secrets[i].path, secrets[j].path) })
		for _, file := range secrets {
			lines = append(lines, fmt.Sprintf("%d %s", file.size, file.path))
		}
	}
	if len(others) > 0 {
		lines = append(lines, "", "## skipped-other")
		sort.SliceStable(others, func(i, j int) bool { return pathLess(others[i].path, others[j].path) })
		for _, file := range others {
			lines = append(lines, fmt.Sprintf("%s %d %s", file.reason, file.size, file.path))
		}
	}
	sort.SliceStable(bogs, func(i, j int) bool { return pathLess(bogs[i].path, bogs[j].path) })
	for _, doc := range bogs {
		lines = append(lines, "", "## bog "+doc.path, doc.xml)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderMembers(members []zipMember) string {
	type xmlDoc struct {
		path string
		xml  string
	}
	type binaryDoc struct {
		path   string
		size   int
		digest string
	}
	var rendered []xmlDoc
	var binaries []binaryDoc
	var problems []string

	ordered := append([]zipMember(nil), members...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return sortKeyLess(ordered[i].name, ordered[j].name)
	})
	for _, member := range ordered {
		k, display := classify(member.name)
		if k != kindShow {
			problems = append(problems, fmt.Sprintf("# skipped %s %s", k, display))
			continue
		}
		if member.declaredSize() > maxEntryBytes {
			problems = append(problems, fmt.Sprintf("# skipped too-large %d %s", member.declaredSize(), display))
			continue
		}
		payload, err := member.read()
		if err != nil {
			problems = append(problems, fmt.Sprintf("# unreadable %s (%s)", display, errClass(err)))
			continue
		}
		if isXMLMember(display, payload) {
			rendered = append(rendered, xmlDoc{path: display, xml: strings.TrimRight(renderXML(payload), "\n")})
		} else {
			binaries = append(binaries, binaryDoc{path: display, size: len(payload), digest: sha256Hex(payload)})
		}
	}
	if len(rendered) == 1 && len(binaries) == 0 && len(problems) == 0 {
		return rendered[0].xml + "\n"
	}
	var lines []string
	if len(rendered) == 0 && len(binaries) == 0 && len(problems) == 0 {
		lines = append(lines, "# niagara-git: bog zip contains no file entries")
	}
	for _, file := range binaries {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines,
			"# binary "+file.path,
			fmt.Sprintf("# size %d", file.size),
			"# sha256 "+file.digest,
		)
	}
	for _, doc := range rendered {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "# entry "+doc.path, doc.xml)
	}
	for _, note := range problems {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, note)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderXML(data []byte) string {
	text, err := prettyXML(data)
	if err != nil {
		return normalizedSource(data, err.Error())
	}
	return text
}

func isXMLMember(display string, payload []byte) bool {
	if strings.HasSuffix(fold(display), ".xml") {
		return true
	}
	// A bog is normally just file.xml. A member with another name that is
	// still XML is shown as XML rather than a checksum.
	return looksLikeXML(payload)
}

func opaque(kind string, data []byte, reason string) string {
	return fmt.Sprintf("# niagara-git: %s not decoded (%s)\n# size %d\n# sha256 %s\n", kind, reason, len(data), sha256Hex(data))
}

func sortKeyLess(a, b string) bool {
	a = strings.ReplaceAll(a, "\\", "/")
	b = strings.ReplaceAll(b, "\\", "/")
	af, bf := fold(a), fold(b)
	if af != bf {
		return af < bf
	}
	return a < b
}

func pathLess(a, b string) bool {
	af, bf := fold(a), fold(b)
	if af != bf {
		return af < bf
	}
	return a < b
}

func noisyCount(noisy map[string][2]uint64) uint64 {
	var n uint64
	for _, cur := range noisy {
		n += cur[0]
	}
	return n
}

func errClass(err error) string {
	switch {
	case errors.Is(err, errEncrypted):
		return "RuntimeError"
	case errors.Is(err, errTooLarge):
		return "ValueError"
	default:
		return "BadZipFile"
	}
}
