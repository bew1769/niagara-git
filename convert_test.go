package main

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSingleFileXMLIsPurePrettyXML(t *testing.T) {
	xml := []byte(`<bajaObjectGraph version="4.0"><p t="b:Folder" n="AHU1" h="2"/></bajaObjectGraph>`)
	text := convertBog(buildZip(t, []zipEntry{{"file.xml", xml}}))
	if !strings.HasPrefix(text, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n") {
		t.Fatalf("missing declaration:\n%s", text)
	}
	if !strings.Contains(text, `<p h="2" n="AHU1" t="b:Folder"/>`) {
		t.Fatalf("missing element:\n%s", text)
	}
	if strings.Contains(text, "file.xml") {
		t.Fatalf("single-member bog should be pure XML:\n%s", text)
	}
	want, err := prettyXML(xml)
	if err != nil {
		t.Fatal(err)
	}
	if text != want {
		t.Fatalf("zip xml != raw xml\n%s\n%s", text, want)
	}
}

func TestRawXMLBog(t *testing.T) {
	xml := []byte(`<bajaObjectGraph version="4.0"><p n="AHU1" h="2" t="b:Folder"/></bajaObjectGraph>`)
	want, err := prettyXML(xml)
	if err != nil {
		t.Fatal(err)
	}
	if convertBog(xml) != want {
		t.Fatal(convertBog(xml))
	}
}

func TestZipMetadataDoesNotAffectBogOutput(t *testing.T) {
	xml := []byte(`<root><p n="A" v="1"/></root>`)
	first := buildZipAt(t, []zipEntry{{"notes.txt", []byte("side")}, {"file.xml", xml}}, time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC))
	second := buildZipAt(t, []zipEntry{{"file.xml", xml}, {"notes.txt", []byte("side")}}, time.Date(1999, 12, 31, 23, 59, 58, 0, time.UTC))
	if bytes.Equal(first, second) {
		t.Fatal("fixture zips unexpectedly identical")
	}
	if convertBog(first) != convertBog(second) {
		t.Fatalf("output changed with zip metadata\n%s\n%s", convertBog(first), convertBog(second))
	}
	text := convertBog(first)
	if !strings.Contains(text, "# entry file.xml") || !strings.Contains(text, "# binary notes.txt") {
		t.Fatal(text)
	}
	if strings.Index(text, "# binary notes.txt") > strings.Index(text, "# entry file.xml") {
		t.Fatal("binaries should be listed before xml entries")
	}
}

func TestCommittedFixturesMatchGolden(t *testing.T) {
	cases := []struct{ src, golden string }{
		{"examples/demoPlant/station/config.bog", "testdata/bog-v1.txt"},
		{"examples/demoPlant/station-v2/config.bog", "testdata/bog-v2.txt"},
		{"examples/demoPlant/backups/demoPlant-v1.dist", "testdata/dist-v1.txt"},
		{"examples/demoPlant/backups/demoPlant-v2.dist", "testdata/dist-v2.txt"},
	}
	for _, tc := range cases {
		raw, err := os.ReadFile(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(tc.golden)
		if err != nil {
			t.Fatal(err)
		}
		var got string
		if strings.HasSuffix(tc.src, ".dist") {
			got = convertDist(raw)
		} else {
			got = convertBog(raw)
		}
		if got != string(want) {
			t.Fatalf("%s textconv mismatch\n--- got ---\n%s\n--- want ---\n%s", tc.src, got, want)
		}
		if strings.Contains(got, "\r") {
			t.Fatalf("%s output contains CR", tc.src)
		}
	}
}

func TestSampleBogShowsPointsAndSyntheticHash(t *testing.T) {
	raw, err := os.ReadFile("examples/demoPlant/station/config.bog")
	if err != nil {
		t.Fatal(err)
	}
	text := convertBog(raw)
	for _, needle := range []string{
		`n="demoPlant"`,
		`n="SupplyTemp"`,
		`n="SupplyFan"`,
		`t="kitControl:GreaterThan"`,
		`v="h:21"`,
		`v="72.0"`,
		"SYNTHETIC-NOT-A-REAL-HASH",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("missing %s", needle)
		}
	}
	if strings.Contains(text, `v="70.0"`) || strings.Contains(text, `n="Occupied"`) {
		t.Fatal("v1 includes v2 changes")
	}
}

func TestVersionTwoChangesSetpointAndLink(t *testing.T) {
	v1, _ := os.ReadFile("examples/demoPlant/station/config.bog")
	v2, _ := os.ReadFile("examples/demoPlant/station-v2/config.bog")
	before, after := convertBog(v1), convertBog(v2)
	if before == after {
		t.Fatal("versions matched")
	}
	for _, needle := range []string{`v="70.0"`, `n="Occupied"`, `v="h:24"`} {
		if !strings.Contains(after, needle) {
			t.Fatalf("v2 missing %s", needle)
		}
	}
}

func TestOpaqueBogIsDeterministic(t *testing.T) {
	data := []byte("not a bog \x00 encrypted-looking")
	first := convertBog(data)
	if first != convertBog(data) {
		t.Fatal("not deterministic")
	}
	if !strings.Contains(first, "passphrase-protected") || !strings.Contains(first, sha256Hex(data)) {
		t.Fatal(first)
	}
	if strings.Contains(first, "\x00") {
		t.Fatal("NUL leaked into output")
	}
}

func TestSampleDistSkipsSecretsAndNoise(t *testing.T) {
	raw, err := os.ReadFile("examples/demoPlant/backups/demoPlant-v1.dist")
	if err != nil {
		t.Fatal(err)
	}
	text := convertDist(raw)
	if text != convertDist(raw) {
		t.Fatal("not deterministic")
	}
	for _, needle := range []string{
		"## bog niagara_user_home/stations/demoPlant/config.bog",
		`v="72.0"`,
		"history count=1 size=",
		"alarm count=1 size=",
		"niagara_user_home/security/keystore.jceks",
		"niagara_user_home/certManagement/demo-client.pem",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("missing %s\n%s", needle, text)
		}
	}
	for _, banned := range []string{
		"BACKUP-SHOULD-NOT-EXPAND",
		"synthetic-history-v1",
		"synthetic-alarm-db",
		"synthetic-keystore-not-a-real-jceks",
		"-----BEGIN CERTIFICATE-----",
		"## bog niagara_user_home/stations/demoPlant/config.backup",
	} {
		if strings.Contains(text, banned) {
			t.Fatalf("leaked %s", banned)
		}
	}
	if strings.Contains(text, sha256Hex([]byte("synthetic-keystore-not-a-real-jceks"))) {
		t.Fatal("keystore checksum was printed")
	}
}

func TestDistEntryOrderDoesNotChangeOutput(t *testing.T) {
	bog := buildZip(t, []zipEntry{{"file.xml", []byte(`<root><p n="A"/></root>`)}})
	entries := []zipEntry{
		{"b/config.bog", bog},
		{"a/shared/note.txt", []byte("hello")},
		{"history/h.bin", []byte("noise")},
		{"security/keystore.jceks", []byte("SECRET")},
	}
	first := buildZipAt(t, entries, time.Date(2018, 5, 1, 0, 0, 0, 0, time.UTC))
	rev := []zipEntry{entries[3], entries[2], entries[1], entries[0]}
	second := buildZipAt(t, rev, time.Date(2024, 8, 9, 10, 11, 12, 0, time.UTC))
	if convertDist(first) != convertDist(second) {
		t.Fatalf("order changed output\n%s\n%s", convertDist(first), convertDist(second))
	}
}

func TestUnsafeMemberIsNotRead(t *testing.T) {
	payload := []byte("SECRET-OUTSIDE")
	bog := buildZip(t, []zipEntry{{"file.xml", []byte(`<root/>`)}})
	data := buildZip(t, []zipEntry{{"../outside.txt", payload}, {"config.bog", bog}})
	text := convertDist(data)
	if strings.Contains(text, string(payload)) || strings.Contains(text, sha256Hex(payload)) {
		t.Fatalf("unsafe payload leaked:\n%s", text)
	}
	if !strings.Contains(text, "unsafe") {
		t.Fatal(text)
	}
}

func TestEncryptedDistIsOpaque(t *testing.T) {
	data := []byte("this is not a zip")
	text := convertDist(data)
	if text != convertDist(data) {
		t.Fatal("not deterministic")
	}
	if !strings.Contains(text, "passphrase-protected") || !strings.Contains(text, sha256Hex(data)) {
		t.Fatal(text)
	}
	if !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
		t.Fatalf("newline mismatch %q", text)
	}
}

type zipEntry struct {
	name string
	data []byte
}

func buildZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	return buildZipAt(t, entries, time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
}

func buildZipAt(t *testing.T, entries []zipEntry, modified time.Time) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate, Modified: modified}
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
