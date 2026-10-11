package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	baseID      = "snapticbase"
	baseVersion = "0.1.0"
	metaFile    = "mod.json"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type version [3]int

type modInfo struct {
	Dir          string   `json:"dir"`
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Version      string   `json:"version"`
	Authors      []string `json:"authors,omitempty"`
	Description  string   `json:"description,omitempty"`
	Dependencies []string `json:"dependencies"`
	Extensions   []string `json:"extensions,omitempty"`
	Main         bool     `json:"main"`
}

func readMeta(root string) (modInfo, error) {
	dir := filepath.Base(root)
	var info modInfo
	raw, err := os.ReadFile(filepath.Join(root, metaFile))
	if errors.Is(err, os.ErrNotExist) {
		return info, fmt.Errorf("%s: %s missing", dir, metaFile)
	}
	if err != nil {
		return info, err
	}
	if err = json.Unmarshal(raw, &info); err != nil {
		return info, fmt.Errorf("%s: %s is not valid json", dir, metaFile)
	}
	info.Dir, info.Main = dir, false
	if !idPattern.MatchString(info.ID) {
		return info, fmt.Errorf("%s: invalid id %q (a-z, 0-9, - _)", dir, info.ID)
	}
	if _, err := parseVersion(info.Version); err != nil {
		return info, fmt.Errorf("%s: %w", dir, err)
	}
	for _, d := range info.Dependencies {
		if _, _, _, err := parseDep(d); err != nil {
			return info, fmt.Errorf("%s: %w", dir, err)
		}
	}
	return info, nil
}

func parseVersion(s string) (version, error) {
	var v version
	t := strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(t, "-+"); i >= 0 {
		t = t[:i]
	}
	parts := strings.Split(t, ".")
	if t == "" || len(parts) > 3 {
		return v, fmt.Errorf("invalid version %q", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, fmt.Errorf("invalid version %q", s)
		}
		v[i] = n
	}
	return v, nil
}

func (a version) cmp(b version) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

var operators = []string{">=", "<=", "==", ">", "<", "="}

func parseDep(s string) (id, op string, v version, err error) {
	id, rest, hasVer := strings.Cut(strings.TrimSpace(s), "@")
	id, rest = strings.TrimSpace(id), strings.TrimSpace(rest)
	if !idPattern.MatchString(id) {
		return "", "", v, fmt.Errorf("invalid dependency %q", s)
	}
	if !hasVer || rest == "" {
		return id, "", v, nil
	}
	op = "="
	for _, o := range operators {
		if after, ok := strings.CutPrefix(rest, o); ok {
			op, rest = o, after
			break
		}
	}
	if op == "==" {
		op = "="
	}
	v, err = parseVersion(rest)
	if err != nil {
		return "", "", v, fmt.Errorf("invalid dependency %q: %w", s, err)
	}
	return id, op, v, nil
}

func satisfies(have version, op string, want version) bool {
	c := have.cmp(want)
	switch op {
	case "":
		return true
	case ">=":
		return c >= 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	case "<":
		return c < 0
	}
	return c == 0
}

func resolve(mods []modInfo) ([]modInfo, error) {
	var problems []error
	byID := map[string]*modInfo{}
	for i := range mods {
		m := &mods[i]
		switch {
		case m.ID == baseID:
			problems = append(problems, fmt.Errorf("%s: id %q is reserved", m.Dir, baseID))
		case byID[m.ID] != nil:
			problems = append(problems, fmt.Errorf("duplicate id %q (%s, %s)", m.ID, byID[m.ID].Dir, m.Dir))
		default:
			byID[m.ID] = m
		}
	}

	needs := map[string][]string{}
	for _, m := range mods {
		for _, d := range m.Dependencies {
			id, op, want, _ := parseDep(d)
			var have string
			switch {
			case id == baseID:
				have = baseVersion
			case byID[id] != nil:
				have = byID[id].Version
				needs[m.ID] = append(needs[m.ID], id)
			default:
				problems = append(problems, fmt.Errorf("%s needs %s (not installed)", m.ID, id))
				continue
			}
			hv, _ := parseVersion(have)
			if !satisfies(hv, op, want) {
				problems = append(problems, fmt.Errorf("%s needs %s (found %s)", m.ID, d, have))
			}
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	var out []modInfo
	done := map[string]bool{}
	for len(out) < len(mods) {
		var ready []string
		for _, m := range mods {
			if done[m.ID] {
				continue
			}
			ok := true
			for _, d := range needs[m.ID] {
				ok = ok && done[d]
			}
			if ok {
				ready = append(ready, m.ID)
			}
		}
		if len(ready) == 0 {
			var stuck []string
			for _, m := range mods {
				if !done[m.ID] {
					stuck = append(stuck, m.ID)
				}
			}
			sort.Strings(stuck)
			return nil, fmt.Errorf("dependency cycle: %s", strings.Join(stuck, ", "))
		}
		sort.Strings(ready)
		out = append(out, *byID[ready[0]])
		done[ready[0]] = true
	}
	return out, nil
}
