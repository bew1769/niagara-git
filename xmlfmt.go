package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

var encodingDecl = regexp.MustCompile(`^<\?xml[^>]*encoding=["']([A-Za-z0-9._-]+)["']`)

// Pretty-printer for the Baja Object Graph XML stored in a .bog zip.
// Component order is preserved. Attribute order and insignificant whitespace
// are normalized so the same station prints the same bytes every time.

const xmlHeader = `<?xml version="1.0" encoding="UTF-8"?>`

type nodeKind int

const (
	kindElem nodeKind = iota
	kindComment
	kindPI
)

type attr struct {
	key string
	val string
}

type node struct {
	kind     nodeKind
	tag      string
	attrs    []attr
	text     string
	tail     string
	children []*node
}

func (n *node) appendText(s string) {
	if len(n.children) == 0 {
		n.text += s
		return
	}
	last := n.children[len(n.children)-1]
	last.tail += s
}

// looksLikeXML reports whether data should be treated as an XML document.
// The trim set matches the historical filter: a UTF-8 BOM and ASCII space.
func looksLikeXML(data []byte) bool {
	head := trimLeftSet(data, "\xef\xbb\xbf \t\r\n")
	return bytes.HasPrefix(head, []byte("<?xml")) || bytes.HasPrefix(head, []byte("<"))
}

func trimLeftSet(data []byte, set string) []byte {
	return bytes.TrimLeft(data, set)
}

// prettyXML returns canonical pretty XML ending with one newline.
func prettyXML(data []byte) (string, error) {
	if hasDoctype(data) {
		return "", fmt.Errorf("DTD or entity declarations are not expanded")
	}
	root, err := parseXML(data)
	if err != nil {
		return "", fmt.Errorf("XML parse error: %s", err.Error())
	}
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteByte('\n')
	writeNode(&b, root, 0)
	b.WriteByte('\n')
	return b.String(), nil
}

func hasDoctype(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	head := bytes.ToLower(data[:n])
	return bytes.Contains(head, []byte("<!doctype")) || bytes.Contains(head, []byte("<!entity"))
}

func normalizedSource(data []byte, reason string) string {
	text := decodeXMLBytes(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return "# niagara-git: " + reason + "\n" + text
}

func parseXML(data []byte) (*node, error) {
	dec := xml.NewDecoder(bytes.NewReader(prepareXMLBytes(data)))
	dec.CharsetReader = charsetReader
	var stack []*node
	var root *node
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{kind: kindElem, tag: xmlTag(t.Name)}
			for _, a := range t.Attr {
				n.attrs = append(n.attrs, attr{key: xmlTag(a.Name), val: a.Value})
			}
			if len(stack) == 0 {
				if root == nil {
					root = n
				}
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected end element")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			// Token bytes are reused on the next call.
			stack[len(stack)-1].appendText(string(append([]byte(nil), t...)))
		case xml.Comment:
			if len(stack) == 0 {
				continue
			}
			c := &node{kind: kindComment, text: string(append([]byte(nil), t...))}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, c)
		case xml.ProcInst:
			if strings.EqualFold(t.Target, "xml") || len(stack) == 0 {
				continue
			}
			body := t.Target
			inst := string(append([]byte(nil), t.Inst...))
			if inst != "" {
				body += " " + strings.TrimLeft(inst, " ")
			}
			p := &node{kind: kindPI, text: body}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, p)
		}
	}
	if root == nil {
		return nil, fmt.Errorf("missing root element")
	}
	return root, nil
}

func xmlTag(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return "{" + name.Space + "}" + name.Local
}

func prepareXMLBytes(data []byte) []byte {
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return data[3:]
	}
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		return []byte(decodeXMLBytes(data))
	}
	return data
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "latin1", "latin-1":
		raw, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		for _, c := range raw {
			b.WriteRune(rune(c))
		}
		return strings.NewReader(b.String()), nil
	case "utf-16", "utf-16le", "utf-16be":
		raw, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		return strings.NewReader(decodeUTF16(raw, strings.ToLower(charset))), nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", charset)
	}
}

func decodeXMLBytes(data []byte) string {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		which := "utf-16le"
		if bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
			which = "utf-16be"
		}
		return decodeUTF16(data, which)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return string(bytes.ToValidUTF8(data[3:], []byte("\uFFFD")))
	}
	enc := "utf-8"
	head := data
	if len(head) > 200 {
		head = data[:200]
	}
	if m := encodingDecl.FindSubmatch(head); m != nil {
		enc = strings.ToLower(string(m[1]))
	}
	switch enc {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	case "iso-8859-1", "latin1", "latin-1":
		var b strings.Builder
		for _, c := range data {
			b.WriteRune(rune(c))
		}
		return b.String()
	case "utf-16", "utf-16le", "utf-16be":
		return decodeUTF16(data, enc)
	default:
		return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	}
}

func decodeUTF16(data []byte, which string) string {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) {
		which = "utf-16le"
		data = data[2:]
	} else if bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		which = "utf-16be"
		data = data[2:]
	}
	if len(data)%2 == 1 {
		data = data[:len(data)-1]
	}
	u := make([]uint16, len(data)/2)
	for i := 0; i < len(u); i++ {
		if which == "utf-16be" {
			u[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
		} else {
			u[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
		}
	}
	return string(utf16.Decode(u))
}

func writeNode(b *strings.Builder, n *node, indent int) {
	pad := strings.Repeat("  ", indent)
	switch n.kind {
	case kindComment:
		body := strings.ReplaceAll(n.text, "--", "- -")
		b.WriteString(pad)
		b.WriteString("<!--")
		b.WriteString(body)
		b.WriteString("-->")
		return
	case kindPI:
		body := strings.NewReplacer("\r", " ", "\n", " ").Replace(n.text)
		b.WriteString(pad)
		b.WriteString("<?")
		b.WriteString(body)
		b.WriteString("?>")
		return
	}

	b.WriteString(pad)
	b.WriteByte('<')
	b.WriteString(n.tag)
	attrs := append([]attr(nil), n.attrs...)
	sort.SliceStable(attrs, func(i, j int) bool { return attrs[i].key < attrs[j].key })
	for _, a := range attrs {
		b.WriteByte(' ')
		b.WriteString(a.key)
		b.WriteString(`="`)
		b.WriteString(escapeAttr(a.val))
		b.WriteByte('"')
	}
	text, hasText := significant(n.text)
	if len(n.children) == 0 && !hasText {
		b.WriteString("/>")
		return
	}
	if len(n.children) == 0 {
		b.WriteByte('>')
		b.WriteString(escapeText(text))
		b.WriteString("</")
		b.WriteString(n.tag)
		b.WriteByte('>')
		return
	}
	b.WriteByte('>')
	if hasText {
		b.WriteByte('\n')
		b.WriteString(pad)
		b.WriteString("  ")
		b.WriteString(escapeText(text))
	}
	for _, child := range n.children {
		b.WriteByte('\n')
		writeNode(b, child, indent+1)
		if tail, ok := significant(child.tail); ok {
			b.WriteByte('\n')
			b.WriteString(pad)
			b.WriteString("  ")
			b.WriteString(escapeText(tail))
		}
	}
	b.WriteByte('\n')
	b.WriteString(pad)
	b.WriteString("</")
	b.WriteString(n.tag)
	b.WriteByte('>')
}

func significant(text string) (string, bool) {
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	return strings.TrimSpace(text), true
}

func escapeText(text string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\r", "&#13;",
		"\n", "&#10;",
	).Replace(text)
}

func escapeAttr(text string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"\r", "&#13;",
		"\n", "&#10;",
		"\t", "&#9;",
	).Replace(text)
}
