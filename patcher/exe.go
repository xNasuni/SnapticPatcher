package main

import (
	"debug/pe"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

type gameExe struct {
	f    *os.File
	pe   *pe.File
	pack *io.SectionReader
	sec  int
}

func (g *gameExe) close() { g.f.Close() }

func openExe(path string) (*gameExe, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	g, err := parseExe(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return g, nil
}

func parseExe(f *os.File) (*gameExe, error) {
	st, _ := f.Stat()
	pf, err := pe.NewFile(f)
	if err != nil {
		return nil, errors.New("not a windows exe")
	}
	for i, s := range pf.Sections {
		if s.Name != "pck" {
			continue
		}
		off, size := int64(s.Offset), int64(s.Size)
		if off+size != st.Size() || size < 12 {
			return nil, errors.New("pck section is not at the end of the exe")
		}
		var tail [12]byte
		f.ReadAt(tail[:], off+size-12)
		length := int64(binary.LittleEndian.Uint64(tail[:8]))
		if binary.LittleEndian.Uint32(tail[8:]) != magic || length <= 0 || length > size-12 {
			return nil, errors.New("bad trailer on the embedded game data")
		}
		return &gameExe{f, pf, io.NewSectionReader(f, off+size-12-length, length), i}, nil
	}
	return nil, errors.New("no game data in this exe")
}

func (g *gameExe) build(path string, p *pack, key []byte) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	off := int64(g.pe.Sections[g.sec].Offset)
	if _, err := io.Copy(out, io.NewSectionReader(g.f, 0, off)); err != nil {
		return err
	}
	n, err := p.write(out, off, g.pack, key)
	if err != nil {
		return err
	}
	var tail [12]byte
	binary.LittleEndian.PutUint64(tail[:], uint64(n))
	binary.LittleEndian.PutUint32(tail[8:], magic)
	if _, err := out.WriteAt(tail[:], off+n); err != nil {
		return err
	}

	var lfanew [4]byte
	g.f.ReadAt(lfanew[:], 0x3c)
	nt := int64(binary.LittleEndian.Uint32(lfanew[:]))
	hdr := nt + 24 + int64(g.pe.FileHeader.SizeOfOptionalHeader) + 40*int64(g.sec)
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(n+12))
	if _, err := out.WriteAt(size[:], hdr+16); err != nil {
		return err
	}
	_, err = out.WriteAt(make([]byte, 4), nt+24+64)
	return err
}
