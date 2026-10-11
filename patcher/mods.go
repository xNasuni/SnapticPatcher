package main

import (
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var extendsPattern = regexp.MustCompile(`(?m)^extends\s+"res://([^"]+)"`)

//go:embed framework/loader.gd
var loaderGD []byte

//go:embed framework/mods_menu.gd
var modsMenuGD []byte

const (
	autoloadKey  = "autoload/SnapticBase"
	autoloadVal  = "*res://" + loaderPath
	modsPrefix   = "snaptic/mods/"
	modsMenuPath = "snaptic/mods_menu.gd"
)

func addAutoload(settings []byte) ([]byte, error) {
	if len(settings) < 8 || string(settings[:4]) != "ECFG" {
		return nil, errors.New("project.binary has an unexpected format")
	}
	out := append([]byte(nil), settings...)
	binary.LittleEndian.PutUint32(out[4:], binary.LittleEndian.Uint32(out[4:])+1)

	val := binary.LittleEndian.AppendUint32(nil, 4)
	val = binary.LittleEndian.AppendUint32(val, uint32(len(autoloadVal)))
	val = append(val, autoloadVal...)
	val = append(val, make([]byte, (4-len(autoloadVal)%4)%4)...)

	out = binary.LittleEndian.AppendUint32(out, uint32(len(autoloadKey)))
	out = append(out, autoloadKey...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(val)))
	return append(out, val...), nil
}

func (p *pack) put(key string, data []byte, flags uint32) bool {
	if i := p.find(key + ".remap"); i >= 0 {
		p.entries = append(p.entries[:i], p.entries[i+1:]...)
	}
	if i := p.find(key); i >= 0 {
		p.entries[i].data, p.entries[i].flags = data, flags
		return true
	}
	p.entries = append(p.entries, &entry{path: p.prefix() + key, data: data, flags: flags})
	return false
}

func applyMods(p *pack, src *io.SectionReader, key []byte, modsDir string) (int, error) {
	var flags uint32
	if p.hasEncryptedFiles() {
		flags = fileEnc
	}

	i := p.find("project.binary")
	if i < 0 {
		return 0, errors.New("project.binary missing from the game data")
	}
	settings, err := p.read(src, p.entries[i], key)
	if err != nil {
		return 0, err
	}
	if settings, err = addAutoload(settings); err != nil {
		return 0, err
	}

	var found []modInfo
	dirs, _ := os.ReadDir(modsDir)
	for _, d := range dirs {
		switch {
		case strings.HasPrefix(d.Name(), "."):
		case !d.IsDir():
			say("skip", grey, d.Name()+" (not a folder)")
		default:
			info, err := readMeta(filepath.Join(modsDir, d.Name()))
			if err != nil {
				return 0, err
			}
			found = append(found, info)
		}
	}
	ordered, err := resolve(found)
	if err != nil {
		return 0, err
	}

	var infos []modInfo
	count := 0
	claimed := map[string]string{}
	for _, m := range ordered {
		info, n, err := installMod(p, src, key, filepath.Join(modsDir, m.Dir), flags, m, claimed)
		if err != nil {
			return 0, err
		}
		infos = append(infos, info)
		count += n
	}

	manifest, _ := json.Marshal(infos)
	p.put(loaderPath, loaderGD, flags)
	p.put(modsMenuPath, modsMenuGD, flags)
	p.put("snaptic/mods.json", manifest, flags)
	p.put("project.binary", settings, flags)
	return count, nil
}

func installMod(p *pack, src *io.SectionReader, key []byte, root string, flags uint32, info modInfo, claimed map[string]string) (modInfo, int, error) {
	say("mod", sky, info.ID+" "+info.Version)
	count := 0

	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		count++

		target, isOverride := strings.CutPrefix(rel, "override/")
		if isOverride {
			if owner, taken := claimed[target]; taken {
				return fmt.Errorf("%s overridden by %s and %s", target, owner, info.ID)
			}
			claimed[target] = info.ID
		}
		if ext, isExt := strings.CutPrefix(rel, "extensions/"); isExt && strings.HasSuffix(ext, ".gd") {
			m := extendsPattern.FindSubmatch(data)
			if m == nil {
				return fmt.Errorf("%s: must extend a res:// script by path", rel)
			}
			base := string(m[1])
			if p.find(base) < 0 && p.find(base+".remap") < 0 {
				return fmt.Errorf("%s: %s not found", rel, base)
			}
			info.Extensions = append(info.Extensions, rel)
			p.put(modsPrefix+info.Dir+"/"+rel, data, flags)
			item("^", sky, rel+" ("+base+")")
			return nil
		}
		if !isOverride {
			info.Main = info.Main || rel == "main.gd"
			p.put(modsPrefix+info.Dir+"/"+rel, data, flags)
			item("+", mint, rel)
			return nil
		}

		if strings.HasSuffix(target, ".png") {
			if i := p.importTarget(target); i >= 0 {
				what, err := p.overrideTexture(src, key, i, data, flags)
				if err != nil {
					return fmt.Errorf("%s: %w", rel, err)
				}
				item("~", peach, what+" (texture)")
				return nil
			}
		}
		if p.put(target, data, flags) {
			item("~", peach, target)
		} else {
			item("+", mint, target)
		}
		return nil
	})
	return info, count, err
}
