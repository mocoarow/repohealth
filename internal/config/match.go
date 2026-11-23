package config

import "github.com/bmatcuk/doublestar/v4"

// AreaOf returns the area holding file, a slash-separated path relative to Root.
func (c Config) AreaOf(file string) (string, bool) {
	area, _, ok := c.locate(file)

	return area, ok
}

// PathOf returns the area path holding file.
func (c Config) PathOf(file string) (string, bool) {
	_, dir, ok := c.locate(file)

	return dir, ok
}

func (c Config) locate(file string) (string, string, bool) {
	if c.Excluded(file) {
		return "", "", false
	}

	for _, area := range c.Areas {
		for _, dir := range area.Paths {
			if contains(dir, file) {
				return area.Name, dir, true
			}
		}
	}

	return "", "", false
}

// Stable reports whether the package directory dir matches a stablePackages pattern.
func (c Config) Stable(dir string) bool {
	return matchAny(c.StablePackages, dir)
}

// Excluded reports whether file matches an exclude pattern.
func (c Config) Excluded(file string) bool {
	return matchAny(c.Exclude, file)
}

func matchAny(patterns []string, name string) bool {
	for _, pattern := range patterns {
		// Load has already rejected invalid patterns.
		if doublestar.MatchUnvalidated(pattern, name) {
			return true
		}
	}

	return false
}
