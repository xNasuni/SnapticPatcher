package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func section(t *testing.T, path string) *io.SectionReader {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	st, _ := f.Stat()
	return io.NewSectionReader(f, 0, st.Size())
}

func TestRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	dir := t.TempDir()
	p := &pack{version: 4, major: 4, minor: 7, flags: flagDirEnc}
	p.entries = []*entry{
		{path: "res://a.gd", data: []byte("hello"), flags: fileEnc},
		{path: "res://b.txt", data: []byte("plain bytes here")},
	}
	first := filepath.Join(dir, "1.pck")
	f, _ := os.Create(first)
	if _, err := p.write(f, 0, nil, key); err != nil {
		t.Fatal(err)
	}
	f.Close()

	src := section(t, first)
	q, err := readPack(src, key)
	if err != nil || len(q.entries) != 2 {
		t.Fatalf("read: %v", err)
	}
	q.entries[1].data = []byte("changed")
	q.entries = append(q.entries, &entry{path: "res://c.gd", data: []byte("new"), flags: fileEnc})
	second := filepath.Join(dir, "2.pck")
	g, _ := os.Create(second)
	if _, err := q.write(g, 100, src, key); err != nil {
		t.Fatal(err)
	}
	g.Close()

	raw, _ := os.ReadFile(second)
	src2 := io.NewSectionReader(bytes.NewReader(raw), 100, int64(len(raw))-100)
	r, err := readPack(src2, key)
	if err != nil || len(r.entries) != 3 {
		t.Fatalf("reread: %v", err)
	}
	src2.Seek(int64(r.fileBase+r.entries[0].ofs), io.SeekStart)
	got, err := unseal(src2, key)
	if err != nil || string(got) != "hello" {
		t.Fatalf("file: %q %v", got, err)
	}
	if _, err := readPack(src2, bytes.Repeat([]byte{1}, 32)); err == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestAddAutoload(t *testing.T) {
	out, err := addAutoload(append([]byte("ECFG"), 0, 0, 0, 0))
	if err != nil || out[4] != 1 || !bytes.Contains(out, []byte(autoloadKey)) || len(out)%4 != 0 {
		t.Fatalf("bad output: %v %x", err, out)
	}
}

func TestPutDropsRemap(t *testing.T) {
	p := &pack{entries: []*entry{{path: "res://a.gd.remap"}, {path: "res://a.gd"}}}
	if !p.put("a.gd", []byte("x"), 0) {
		t.Fatal("expected replace")
	}
	if p.find("a.gd.remap") >= 0 || len(p.entries) != 1 {
		t.Fatal("remap was not dropped")
	}
	if p.put("b.gd", []byte("y"), 0) || p.find("b.gd") < 0 {
		t.Fatal("expected add")
	}
}
