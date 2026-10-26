package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a release's SemVer core: major.minor.patch. A pre-release
// suffix ("-rc.1") sorts before the same core without one; two pre-releases
// compare as text, enough for the rc.N this project tags.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// ParseVersion reads "1.5.0", "v1.5.0" or "1.5.0-rc.1".
func ParseVersion(s string) (Version, error) {
	core, pre, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(s), "v"), "-")
	core, _, _ = strings.Cut(core, "+") // build metadata never orders
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("version %q is not major.minor.patch", s)
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return Version{}, fmt.Errorf("version %q is not major.minor.patch", s)
		}
		n[i] = v
	}
	return Version{Major: n[0], Minor: n[1], Patch: n[2], Pre: pre}, nil
}

// Compare is negative when a is older than b, zero when equal, positive when newer.
func Compare(a, b Version) int {
	for _, d := range [3]int{a.Major - b.Major, a.Minor - b.Minor, a.Patch - b.Patch} {
		if d != 0 {
			return d
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	}
	return strings.Compare(a.Pre, b.Pre)
}
