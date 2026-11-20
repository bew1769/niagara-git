package main

import "testing"

func TestPrettySortsAttributesAndIndents(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?><bajaObjectGraph version="4.0"><p t="b:Folder" n="AHU1" h="2"><!-- supply air --><p n="SupplyTemp" t="control:NumericWritable" h="3"><p v="72.0" n="fallback"/></p></p></bajaObjectGraph>`)
	want := `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
  <p h="2" n="AHU1" t="b:Folder">
    <!-- supply air -->
    <p h="3" n="SupplyTemp" t="control:NumericWritable">
      <p n="fallback" v="72.0"/>
    </p>
  </p>
</bajaObjectGraph>
`
	got, err := prettyXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("pretty XML mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPrettyPreservesElementOrder(t *testing.T) {
	raw := []byte(`<root><p n="second"/><p n="first"/></root>`)
	text, err := prettyXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if i, j := indexOf(text, `n="second"`), indexOf(text, `n="first"`); i > j {
		t.Fatalf("element order changed: second at %d, first at %d", i, j)
	}
}

func TestPrettyEscapesAttributeText(t *testing.T) {
	raw := []byte(`<root><p n="note" v="a&amp;b&quot;c&lt;d"/></root>`)
	text, err := prettyXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(text, `v="a&amp;b&quot;c&lt;d"`) {
		t.Fatalf("escape mismatch:\n%s", text)
	}
}

func TestPrettyIdempotent(t *testing.T) {
	raw := []byte(`<root><p n="A" h="1"><p v="x" n="b"/></p></root>`)
	once, err := prettyXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := prettyXML([]byte(once))
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Fatalf("not idempotent\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

func TestPrettyCRLFAndAttributeOrder(t *testing.T) {
	a := []byte(`<root><p b="1" a="2"/></root>`)
	b := []byte("<root>\r\n  <p a=\"2\" b=\"1\"/>\r\n</root>")
	left, err := prettyXML(a)
	if err != nil {
		t.Fatal(err)
	}
	right, err := prettyXML(b)
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("CRLF normalization mismatch\n%s\n%s", left, right)
	}
	if contains(left, "\r") {
		t.Fatal("output contains CR")
	}
}

func TestPrettyRefusesDoctype(t *testing.T) {
	raw := []byte(`<!DOCTYPE bajaObjectGraph [<!ENTITY x "y">]><root>&x;</root>`)
	if _, err := prettyXML(raw); err == nil {
		t.Fatal("expected DTD to be refused")
	}
	text := convertBog(raw)
	if !contains(text, "DTD or entity declarations are not expanded") {
		t.Fatalf("convert path did not explain the refusal:\n%s", text)
	}
	if !contains(text, "<!DOCTYPE") {
		t.Fatalf("source was dropped:\n%s", text)
	}
}

func TestPrettyNewlineInAttributeStaysOnOneLine(t *testing.T) {
	raw := []byte(`<root><p v="line1&#10;line2"/></root>`)
	text, err := prettyXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(text, "&#10;") {
		t.Fatalf("newline was not escaped:\n%s", text)
	}
	for _, line := range splitLines(text) {
		if contains(line, "line1") && contains(line, "\n") {
			t.Fatal("attribute value split across lines")
		}
	}
}

func indexOf(s, sub string) int {
	return len([]rune(s[:index(s, sub)]))
}

func index(s, sub string) int {
	i := 0
	for {
		if i > len(s)-len(sub) {
			return len(s)
		}
		if s[i:i+len(sub)] == sub {
			return i
		}
		i++
	}
}

func contains(s, sub string) bool {
	return index(s, sub) < len(s)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
