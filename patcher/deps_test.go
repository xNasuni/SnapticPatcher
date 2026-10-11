package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestDepSpec(t *testing.T) {
	cases := []struct {
		dep, have string
		ok        bool
	}{
		{"snapticbase", "0.1.0", true},
		{"snapticbase@>=0.1.0", "0.1.0", true},
		{"snapticbase@>=0.2", "0.1.0", false},
		{"snapticbase@<1", "0.9.9", true},
		{"snapticbase@0.1.0", "0.1.0", true},
		{"snapticbase@=0.1.1", "0.1.0", false},
		{"snapticbase@>0.1.0", "0.1.0", false},
	}
	for _, c := range cases {
		_, op, want, err := parseDep(c.dep)
		have, _ := parseVersion(c.have)
		if err != nil || satisfies(have, op, want) != c.ok {
			t.Errorf("%s vs %s: want %v (err %v)", c.dep, c.have, c.ok, err)
		}
	}
	if _, _, _, err := parseDep("Bad Id@1"); err == nil {
		t.Error("bad id accepted")
	}
	if _, err := parseVersion("1.x"); err == nil {
		t.Error("bad version accepted")
	}
}

func TestResolveOrder(t *testing.T) {
	mods := []modInfo{
		{Dir: "z", ID: "zeta", Version: "1.0.0", Dependencies: []string{"alpha@>=2"}},
		{Dir: "a", ID: "alpha", Version: "2.1.0", Dependencies: []string{"snapticbase@>=0.1.0"}},
		{Dir: "m", ID: "mid", Version: "0.0.0"},
	}
	out, err := resolve(mods)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range out {
		ids = append(ids, m.ID)
	}
	if got := strings.Join(ids, ","); got != "alpha,mid,zeta" {
		t.Fatalf("order %s", got)
	}
}

func TestResolveErrors(t *testing.T) {
	bad := map[string][]modInfo{
		"not installed": {{Dir: "a", ID: "a", Version: "0.0.0", Dependencies: []string{"ghost"}}},
		"found":         {{Dir: "a", ID: "a", Version: "0.0.0", Dependencies: []string{"snapticbase@>=1.0.0"}}},
		"duplicate id":  {{Dir: "a", ID: "x", Version: "0.0.0"}, {Dir: "b", ID: "x", Version: "0.0.0"}},
		"reserved":      {{Dir: "a", ID: baseID, Version: "0.0.0"}},
		"cycle": {
			{Dir: "a", ID: "a", Version: "0.0.0", Dependencies: []string{"b"}},
			{Dir: "b", ID: "b", Version: "0.0.0", Dependencies: []string{"a"}},
		},
	}
	for want, mods := range bad {
		if _, err := resolve(mods); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}

func TestLoaderVersionMatches(t *testing.T) {
	if !strings.Contains(string(loaderGD), `const VERSION := "`+baseVersion+`"`) {
		t.Fatal("framework/loader.gd VERSION does not match baseVersion")
	}
}

func TestExtendsPattern(t *testing.T) {
	src := []byte("# note\nclass_name Foo\nextends \"res://ui/menu.gd\"\n")
	if m := extendsPattern.FindSubmatch(src); m == nil || string(m[1]) != "ui/menu.gd" {
		t.Fatalf("no match: %v", m)
	}
	if extendsPattern.Match([]byte("extends Node\n")) {
		t.Fatal("matched a class extends")
	}
}

func TestCtexPacksPng(t *testing.T) {
	var src bytes.Buffer
	if err := png.Encode(&src, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	out, err := ctex(src.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	const dataOfs = 56
	if string(out[dataOfs:dataOfs+4]) != "PNG " || !bytes.Equal(out[dataOfs+4:], src.Bytes()) {
		t.Fatal("png data is not prefixed with the \"PNG \" marker")
	}
	if n := binary.LittleEndian.Uint32(out[dataOfs-4:]); int(n) != src.Len()+4 {
		t.Fatalf("size %d, want %d", n, src.Len()+4)
	}
}
