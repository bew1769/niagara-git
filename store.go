package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Clean/smudge container for a .bog zip.
//
// The blob records every zip entry in central-directory order: the name and
// the exact uncompressed bytes. Zip timestamps and compression are not stored,
// so rewriting the zip wrapper does not change the git object. smudge rebuilds
// a zip Niagara 4 can open.
//
// .dist archives stay zip blobs. A distribution file bundles history databases,
// alarm databases, and keystores. Expanding one would commit those bytes and
// would change on every backup. Diff them with textconv only.

const storeMagic = "NSTORE1\n"

var storeFixedTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

type storedEntry struct {
	name string
	data []byte
}

func isStore(data []byte) bool {
	return bytes.HasPrefix(data, []byte(storeMagic))
}

func clean(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("niagara-git: clean: empty input")
	}
	if isStore(data) {
		comment, entries, err := parseStore(data)
		if err != nil {
			return nil, err
		}
		return encodeStore(comment, entries), nil
	}
	if !bytes.HasPrefix(data, []byte("PK")) {
		return nil, fmt.Errorf("niagara-git: clean: input is not a .bog zip or a niagara-git store")
	}
	comment, entries, err := readZipEntries(data)
	if err != nil {
		return nil, fmt.Errorf("niagara-git: clean: %w", err)
	}
	return encodeStore(comment, entries), nil
}

func smudge(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("niagara-git: smudge: empty input")
	}
	if bytes.HasPrefix(data, []byte("PK")) {
		// A commit made before --store-xml still holds a zip.
		return append([]byte(nil), data...), nil
	}
	if !isStore(data) {
		return nil, fmt.Errorf("niagara-git: smudge: input is not a niagara-git store or a .bog zip")
	}
	comment, entries, err := parseStore(data)
	if err != nil {
		return nil, err
	}
	return buildStoredZip(comment, entries)
}

func readZipEntries(data []byte) (string, []storedEntry, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, fmt.Errorf("not a readable zip archive")
	}
	entries := make([]storedEntry, 0, len(reader.File))
	for _, file := range reader.File {
		if file.Flags&1 != 0 {
			return "", nil, fmt.Errorf("encrypted entry %s", file.Name)
		}
		if file.UncompressedSize64 > maxEntryBytes {
			return "", nil, fmt.Errorf("entry %s larger than %d bytes", file.Name, maxEntryBytes)
		}
		payload, err := readZipFile(file)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", file.Name, err)
		}
		entries = append(entries, storedEntry{name: file.Name, data: payload})
	}
	return reader.Comment, entries, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	if strings.HasSuffix(file.Name, "/") || file.FileInfo().IsDir() {
		return []byte{}, nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	payload, err := io.ReadAll(io.LimitReader(rc, maxEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(payload)) > maxEntryBytes {
		return nil, fmt.Errorf("larger than %d bytes", maxEntryBytes)
	}
	return payload, nil
}

func encodeStore(comment string, entries []storedEntry) []byte {
	var b bytes.Buffer
	b.WriteString(storeMagic)
	if comment != "" {
		raw := []byte(comment)
		fmt.Fprintf(&b, "comment %d\n", len(raw))
		b.Write(raw)
	}
	fmt.Fprintf(&b, "entries %d\n", len(entries))
	for _, entry := range entries {
		enc, body := encodeBody(entry.data)
		name := []byte(entry.name)
		fmt.Fprintf(&b, "entry %d %d %s\n", len(name), len(body), enc)
		b.Write(name)
		b.Write(body)
	}
	return b.Bytes()
}

func encodeBody(data []byte) (string, []byte) {
	if bytes.IndexByte(data, 0) < 0 && utf8.Valid(data) {
		return "raw", data
	}
	return "b64", []byte(base64.StdEncoding.EncodeToString(data))
}

func parseStore(data []byte) (string, []storedEntry, error) {
	if !isStore(data) {
		return "", nil, fmt.Errorf("niagara-git: not a store blob")
	}
	r := storeReader{b: data, i: len(storeMagic)}
	line, err := r.line()
	if err != nil {
		return "", nil, fmt.Errorf("niagara-git: store: %w", err)
	}
	var comment string
	if strings.HasPrefix(line, "comment ") {
		n, err := strconv.Atoi(strings.TrimPrefix(line, "comment "))
		if err != nil || n < 0 {
			return "", nil, fmt.Errorf("niagara-git: store: bad comment length")
		}
		raw, err := r.take(n)
		if err != nil {
			return "", nil, fmt.Errorf("niagara-git: store: %w", err)
		}
		comment = string(raw)
		line, err = r.line()
		if err != nil {
			return "", nil, fmt.Errorf("niagara-git: store: %w", err)
		}
	}
	if !strings.HasPrefix(line, "entries ") {
		return "", nil, fmt.Errorf("niagara-git: store: missing entries header")
	}
	count, err := strconv.Atoi(strings.TrimPrefix(line, "entries "))
	if err != nil || count < 0 {
		return "", nil, fmt.Errorf("niagara-git: store: bad entry count")
	}
	entries := make([]storedEntry, 0, count)
	for i := 0; i < count; i++ {
		header, err := r.line()
		if err != nil {
			return "", nil, fmt.Errorf("niagara-git: store: %w", err)
		}
		var nameLen, bodyLen int
		var enc string
		if _, err := fmt.Sscanf(header, "entry %d %d %s", &nameLen, &bodyLen, &enc); err != nil {
			return "", nil, fmt.Errorf("niagara-git: store: bad entry header %q", header)
		}
		if nameLen < 0 || bodyLen < 0 {
			return "", nil, fmt.Errorf("niagara-git: store: negative length")
		}
		name, err := r.take(nameLen)
		if err != nil {
			return "", nil, fmt.Errorf("niagara-git: store: %w", err)
		}
		body, err := r.take(bodyLen)
		if err != nil {
			return "", nil, fmt.Errorf("niagara-git: store: %w", err)
		}
		switch enc {
		case "raw":
		case "b64":
			decoded, err := base64.StdEncoding.DecodeString(string(body))
			if err != nil {
				return "", nil, fmt.Errorf("niagara-git: store: %s: %w", name, err)
			}
			body = decoded
		default:
			return "", nil, fmt.Errorf("niagara-git: store: unknown encoding %s", enc)
		}
		entries = append(entries, storedEntry{name: string(name), data: body})
	}
	if r.i != len(r.b) {
		return "", nil, fmt.Errorf("niagara-git: store: %d trailing bytes", len(r.b)-r.i)
	}
	return comment, entries, nil
}

type storeReader struct {
	b []byte
	i int
}

func (r *storeReader) line() (string, error) {
	if r.i >= len(r.b) {
		return "", io.ErrUnexpectedEOF
	}
	rel := bytes.IndexByte(r.b[r.i:], '\n')
	if rel < 0 {
		return "", io.ErrUnexpectedEOF
	}
	s := string(r.b[r.i : r.i+rel])
	r.i += rel + 1
	return s, nil
}

func (r *storeReader) take(n int) ([]byte, error) {
	if n < 0 || r.i+n > len(r.b) {
		return nil, io.ErrUnexpectedEOF
	}
	out := append([]byte(nil), r.b[r.i:r.i+n]...)
	r.i += n
	return out, nil
}

func buildStoredZip(comment string, entries []storedEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if comment != "" {
		if err := zw.SetComment(comment); err != nil {
			return nil, err
		}
	}
	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:     entry.name,
			Method:   zip.Deflate,
			Modified: storeFixedTime,
		}
		if strings.HasSuffix(entry.name, "/") {
			header.Method = zip.Store
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := writer.Write(entry.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func entriesEqual(left, right []byte) error {
	lc, le, err := readZipEntries(left)
	if err != nil {
		return err
	}
	rc, re, err := readZipEntries(right)
	if err != nil {
		return err
	}
	if lc != rc {
		return fmt.Errorf("zip comment %q != %q", lc, rc)
	}
	if len(le) != len(re) {
		return fmt.Errorf("entry count %d != %d", len(le), len(re))
	}
	for i := range le {
		if le[i].name != re[i].name {
			return fmt.Errorf("entry %d name %q != %q", i, le[i].name, re[i].name)
		}
		if !bytes.Equal(le[i].data, re[i].data) {
			return fmt.Errorf("entry %s contents differ (%d vs %d bytes)", le[i].name, len(le[i].data), len(re[i].data))
		}
	}
	return nil
}
