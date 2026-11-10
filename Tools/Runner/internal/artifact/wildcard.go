package artifact

import (
	"io/fs"
	"sort"
	"strings"
)

// ExpandWildcard lists, in per-segment lexical order, every existing path in
// fsys that matches the slash-separated pattern. Only '*' is special: it
// matches any text within one path segment and never crosses '/'. Every other
// character, including '?', '[', ']' and '\', matches only itself. A pattern
// without '*' is returned as []string{pattern} whether or not it exists. No
// match gives an empty result, never an error.
//
// Every segment before the last must name a directory; the last may name a
// file or a directory. A pattern with an empty, "." or ".." segment lies
// outside the root and matches nothing.
func ExpandWildcard(fsys fs.FS, pattern string) []string {
	if !strings.Contains(pattern, "*") {
		return []string{pattern}
	}
	segments := strings.Split(pattern, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return nil
		}
	}
	candidates := []string{""}
	for _, seg := range segments {
		var next []string
		for _, base := range candidates {
			next = append(next, expandSegment(fsys, base, seg)...)
		}
		candidates = next
	}
	return candidates
}

// expandSegment returns the existing children of base whose name matches seg,
// as paths from the root, sorted lexically.
func expandSegment(fsys fs.FS, base, seg string) []string {
	if !strings.Contains(seg, "*") {
		p := joinSlash(base, seg)
		if _, err := fs.Stat(fsys, p); err != nil {
			return nil
		}
		return []string{p}
	}
	dir := base
	if dir == "" {
		dir = "."
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if matchStar(seg, e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = joinSlash(base, n)
	}
	return out
}

// matchStar reports whether name matches pattern, where '*' stands for any
// (possibly empty) text and every other character matches only itself.
func matchStar(pattern, name string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name
	}
	first, last := parts[0], parts[len(parts)-1]
	if len(name) < len(first)+len(last) || !strings.HasPrefix(name, first) || !strings.HasSuffix(name, last) {
		return false
	}
	rest := name[len(first) : len(name)-len(last)]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(rest, mid)
		if i < 0 {
			return false
		}
		rest = rest[i+len(mid):]
	}
	return true
}

func joinSlash(base, name string) string {
	if base == "" {
		return name
	}
	return base + "/" + name
}
