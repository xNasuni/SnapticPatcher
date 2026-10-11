package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"io"
	"regexp"
	"strings"
) 

var importPath = regexp.MustCompile(`(?m)^path[\w.]*="res://([^"]+\.ctex)"`)

func (p *pack) read(src *io.SectionReader, e *entry, key []byte) ([]byte, error) {
	if e.data != nil {
		return e.data, nil
	}
	if _, err := src.Seek(int64(p.fileBase+e.ofs), io.SeekStart); err != nil {
		return nil, err
	}
	if e.flags&fileEnc != 0 {
		return unseal(src, key)
	}
	buf := make([]byte, e.size)
	_, err := io.ReadFull(src, buf)
	return buf, err
}

func ctex(pngData []byte) ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("not a valid png: %w", err)
	}
	var b bytes.Buffer
	b.WriteString("GST2")
	le := func(v ...uint32) { binary.Write(&b, binary.LittleEndian, v) }
	le(1, uint32(cfg.Width), uint32(cfg.Height), 0, 0xFFFFFFFF, 0, 0, 0)
	le(1)
	binary.Write(&b, binary.LittleEndian, []uint16{uint16(cfg.Width), uint16(cfg.Height)})
	le(0, 5, uint32(len(pngData)+4))
	b.WriteString("PNG ")
	b.Write(pngData)
	return b.Bytes(), nil
}

func (p *pack) importTarget(rel string) int {
	if i := p.find(rel + ".import"); i >= 0 {
		return i
	}
	stem := strings.TrimSuffix(rel, ".png") + "."
	for i, e := range p.entries {
		if k := e.key(); strings.HasPrefix(k, stem) && strings.HasSuffix(k, ".import") {
			return i
		}
	}
	return -1
}

func (p *pack) overrideTexture(src *io.SectionReader, key []byte, i int, pngData []byte, flags uint32) (string, error) {
	imp, err := p.read(src, p.entries[i], key)
	if err != nil {
		return "", err
	}
	m := importPath.FindSubmatch(imp)
	if m == nil {
		return "", fmt.Errorf("no imported texture for %s", p.entries[i].key())
	}
	j := p.find(string(m[1]))
	if j < 0 {
		return "", fmt.Errorf("%s missing from the game data", m[1])
	}
	data, err := ctex(pngData)
	if err != nil {
		return "", err
	}
	p.entries[j].data, p.entries[j].flags = data, flags
	return strings.TrimSuffix(p.entries[i].key(), ".import"), nil
}
