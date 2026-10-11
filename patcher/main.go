package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	exeName    = "snaptic.exe"
	origName   = "snaptic_original.exe"
	loaderPath = "snaptic/loader.gd"
)

const (
	reset = "\x1b[0m"
	grey  = "\x1b[38;2;140;140;158m"
	pink  = "\x1b[38;2;245;169;190m"
	mint  = "\x1b[38;2;166;227;190m"
	peach = "\x1b[38;2;250;204;163m"
	sky   = "\x1b[38;2;163;205;245m"
	lilac = "\x1b[38;2;203;182;242m"
)

func say(label, color, msg string) { fmt.Printf("  %s%-8s%s %s\n", color, label, reset, msg) }

func item(sym, color, msg string) { fmt.Printf("    %s%s%s %s\n", color, sym, reset, msg) }

func main() {
	enableColor()
	dirFlag := flag.String("dir", "", "game folder (default: next to this exe)")
	noPause := flag.Bool("no-pause", false, "don't wait for Enter before exiting")
	flag.Parse()

	dir := *dirFlag
	if dir == "" {
		exe, _ := os.Executable()
		dir = filepath.Dir(exe)
	}

	fmt.Printf("%ssnaptic patcher%s\n", lilac, reset)
	var err error
	if flag.Arg(0) == "restore" {
		err = restore(dir)
	} else {
		err = patch(dir)
	}
	code := 0
	if err != nil {
		fmt.Printf("  %s%-8s%s %v\n", pink, "error", reset, err)
		code = 1
	}
	if !*noPause {
		fmt.Printf("\n%spress enter to exit%s", grey, reset)
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(code)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func readKey(dir string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "key.txt"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("key.txt not found (64 character hex aes decryption key for the game)")
	} else if err != nil {
		return nil, err
	}
	s := strings.TrimSpace(string(raw))
	if len(s) != 64 {
		return nil, fmt.Errorf("key.txt needs 64 hex characters, found %d", len(s))
	}
	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.New("key.txt is not valid hex")
	}
	return key, nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(to)
		return err
	}
	return out.Close()
}

func explain(err error) error {
	if errors.Is(err, errBadKey) {
		return errors.New("wrong key, could not decrypt the game data")
	}
	return err
}

func restore(dir string) error {
	orig, target := filepath.Join(dir, origName), filepath.Join(dir, exeName)
	if !exists(orig) {
		return fmt.Errorf("%s not found", origName)
	}
	if err := copyFile(orig, target); err != nil {
		return fmt.Errorf("could not restore %s (game running?): %w", exeName, err)
	}
	say("restored", mint, exeName)
	return nil
}

func isPatched(path string, key []byte) (bool, error) {
	g, err := openExe(path)
	if err != nil {
		return false, err
	}
	defer g.close()
	p, err := readPack(g.pack, key)
	if err != nil {
		return false, explain(err)
	}
	return p.find(loaderPath) >= 0, nil
}

func patch(dir string) error {
	key, err := readKey(dir)
	if err != nil {
		return err
	}
	say("key", mint, "ok")

	target, orig := filepath.Join(dir, exeName), filepath.Join(dir, origName)
	if !exists(target) {
		return fmt.Errorf("%s not found (run the patcher in the game folder)", exeName)
	}

	patched, err := isPatched(target, key)
	switch {
	case err != nil:
		return err
	case !patched:
		if err := copyFile(target, orig); err != nil {
			return err
		}
		say("backup", mint, "saved "+origName)
	case !exists(orig):
		return fmt.Errorf("%s is patched but %s is gone (verify game files on steam)", exeName, origName)
	default:
		say("backup", grey, "using "+origName)
	}

	g, err := openExe(orig)
	if err != nil {
		return err
	}
	defer g.close()
	p, err := readPack(g.pack, key)
	if err != nil {
		return explain(err)
	}
	say("pack", grey, fmt.Sprintf("godot %d.%d.%d, %d files", p.major, p.minor, p.patch, len(p.entries)))

	n, err := applyMods(p, g.pack, key, filepath.Join(dir, "mods"))
	if err != nil {
		return err
	}
	if n == 0 {
		say("mods", peach, "none found")
	}

	tmp := target + ".tmp"
	if err = g.build(tmp, p, key); err == nil {
		err = os.Rename(tmp, target)
	}
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("could not write %s (game running?): %w", exeName, err)
	}
	st, _ := os.Stat(target)
	say("done", mint, fmt.Sprintf("%s%s%s patched (%.1f MB)", sky, exeName, reset, float64(st.Size())/1e6))
	return nil
}
