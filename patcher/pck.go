package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	magic       = 0x43504447
	flagDirEnc  = 1
	flagSparse  = 4
	fileEnc     = 1
	fileRemoved = 2
	fileDelta   = 4
	align       = 16
	headerLen   = 104
	resPrefix   = "res://"
)

var errBadKey = errors.New("decryption failed")

type entry struct {
	path  string
	ofs   uint64
	size  uint64
	md5   [16]byte
	flags uint32
	data  []byte
}

type pack struct {
	version, major, minor, patch, flags uint32
	fileBase                            uint64
	entries                             []*entry
}

func (e *entry) key() string { return strings.TrimPrefix(e.path, resPrefix) }

func (e *entry) rawLen() uint64 {
	switch {
	case e.flags&fileRemoved != 0:
		return 0
	case e.flags&fileEnc != 0:
		return 40 + (e.size+15)/16*16
	}
	return e.size
}

func (p *pack) hasEncryptedFiles() bool {
	for _, e := range p.entries {
		if e.flags&fileEnc != 0 {
			return true
		}
	}
	return false
}

func (p *pack) find(key string) int {
	for i, e := range p.entries {
		if e.key() == key {
			return i
		}
	}
	return -1
}

func (p *pack) prefix() string {
	for _, e := range p.entries {
		if strings.HasPrefix(e.path, resPrefix) {
			return resPrefix
		}
	}
	return ""
}

func seal(plain, key []byte) []byte {
	iv := make([]byte, 16)
	rand.Read(iv)
	sum := md5.Sum(plain)
	buf := make([]byte, (len(plain)+15)/16*16)
	copy(buf, plain)
	blk, _ := aes.NewCipher(key)
	cipher.NewCFBEncrypter(blk, iv).XORKeyStream(buf, buf)

	out := make([]byte, 0, 40+len(buf))
	out = append(out, sum[:]...)
	out = binary.LittleEndian.AppendUint64(out, uint64(len(plain)))
	out = append(out, iv...)
	return append(out, buf...)
}

func unseal(r io.Reader, key []byte) ([]byte, error) {
	var h struct {
		MD5 [16]byte
		Len uint64
		IV  [16]byte
	}
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if h.Len > 1<<31 {
		return nil, errBadKey
	}
	buf := make([]byte, (h.Len+15)/16*16)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	blk, _ := aes.NewCipher(key)
	cipher.NewCFBDecrypter(blk, h.IV[:]).XORKeyStream(buf, buf)
	buf = buf[:h.Len]
	if md5.Sum(buf) != h.MD5 {
		return nil, errBadKey
	}
	return buf, nil
}

func readPack(f *io.SectionReader, key []byte) (*pack, error) {
	var h struct {
		Magic, Version, Major, Minor, Patch, Flags uint32
		FileBase, DirOfs                           uint64
	}
	if err := binary.Read(f, binary.LittleEndian, &h); err != nil || h.Magic != magic {
		return nil, errors.New("not a godot pack")
	}
	if h.Version != 3 && h.Version != 4 {
		return nil, fmt.Errorf("unsupported pack version %d", h.Version)
	}
	if h.Flags&flagSparse != 0 {
		return nil, errors.New("sparse packs are not supported")
	}
	p := &pack{h.Version, h.Major, h.Minor, h.Patch, h.Flags, h.FileBase, nil}

	if _, err := f.Seek(int64(h.DirOfs), io.SeekStart); err != nil {
		return nil, err
	}
	br := bufio.NewReader(f)
	var count uint32
	binary.Read(br, binary.LittleEndian, &count)

	var dir io.Reader = br
	if h.Flags&flagDirEnc != 0 {
		plain, err := unseal(br, key)
		if err != nil {
			return nil, err
		}
		dir = bytes.NewReader(plain)
	}

	for i := uint32(0); i < count; i++ {
		var sl uint32
		if err := binary.Read(dir, binary.LittleEndian, &sl); err != nil {
			return nil, errors.New("corrupt pack directory")
		}
		name := make([]byte, sl)
		io.ReadFull(dir, name)
		e := &entry{path: strings.TrimRight(string(name), "\x00")}
		var tail struct {
			Ofs, Size uint64
			MD5       [16]byte
			Flags     uint32
		}
		if err := binary.Read(dir, binary.LittleEndian, &tail); err != nil {
			return nil, errors.New("corrupt pack directory")
		}
		e.ofs, e.size, e.md5, e.flags = tail.Ofs, tail.Size, tail.MD5, tail.Flags
		if e.flags&fileDelta != 0 {
			return nil, errors.New("delta packs are not supported")
		}
		p.entries = append(p.entries, e)
	}
	return p, nil
}

func (p *pack) write(out *os.File, base int64, src *io.SectionReader, key []byte) (int64, error) {
	if _, err := out.Seek(base, io.SeekStart); err != nil {
		return 0, err
	}
	w := bufio.NewWriterSize(out, 1<<20)
	var pos uint64
	pad := func() {
		if n := (align - pos%align) % align; n > 0 {
			w.Write(make([]byte, n))
			pos += n
		}
	}
	w.Write(make([]byte, headerLen))
	pos = headerLen
	pad()
	fileBase := pos

	for _, e := range p.entries {
		start := pos
		if e.data != nil {
			sum := md5.Sum(e.data)
			e.md5, e.size = sum, uint64(len(e.data))
			blob := e.data
			if e.flags&fileEnc != 0 {
				blob = seal(e.data, key)
			}
			w.Write(blob)
			pos += uint64(len(blob))
		} else if n := e.rawLen(); n > 0 {
			if _, err := io.Copy(w, io.NewSectionReader(src, int64(p.fileBase+e.ofs), int64(n))); err != nil {
				return 0, err
			}
			pos += n
		}
		e.ofs = start - fileBase
		pad()
	}

	var dir bytes.Buffer
	for _, e := range p.entries {
		padn := (4 - len(e.path)%4) % 4
		binary.Write(&dir, binary.LittleEndian, uint32(len(e.path)+padn))
		dir.WriteString(e.path)
		dir.Write(make([]byte, padn))
		binary.Write(&dir, binary.LittleEndian, e.ofs)
		binary.Write(&dir, binary.LittleEndian, e.size)
		dir.Write(e.md5[:])
		binary.Write(&dir, binary.LittleEndian, e.flags)
	}

	dirOfs := pos
	binary.Write(w, binary.LittleEndian, uint32(len(p.entries)))
	dirBytes := dir.Bytes()
	if p.flags&flagDirEnc != 0 {
		dirBytes = seal(dirBytes, key)
	}
	w.Write(dirBytes)
	if err := w.Flush(); err != nil {
		return 0, err
	}

	var hdr bytes.Buffer
	binary.Write(&hdr, binary.LittleEndian, []uint32{magic, p.version, p.major, p.minor, p.patch, p.flags})
	binary.Write(&hdr, binary.LittleEndian, []uint64{fileBase, dirOfs})
	_, err := out.WriteAt(hdr.Bytes(), base)
	return int64(dirOfs) + 4 + int64(len(dirBytes)), err
}
