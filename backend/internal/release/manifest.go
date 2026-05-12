package release

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Manifest struct {
	SchemaVersion          int              `json:"schemaVersion"`
	AppVersion             string           `json:"appVersion"`
	MinimumResourceVersion string           `json:"minimumResourceVersion"`
	Packages               []RuntimePackage `json:"packages"`
	Files                  []RuntimeFile    `json:"files,omitempty"`
}

type RuntimePackage struct {
	ID       string `json:"id"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256,omitempty"`
	Path     string `json:"path,omitempty"`
	Note     string `json:"note,omitempty"`
}

type RuntimeFile struct {
	Path    string   `json:"path"`
	SHA256  string   `json:"sha256"`
	Targets []string `json:"targets"`
	Note    string   `json:"note,omitempty"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != 2 {
		return Manifest{}, fmt.Errorf("unsupported manifest schema: %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func (m Manifest) RequiredPackages(target string) ([]RuntimePackage, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("target is required")
	}

	var out []RuntimePackage
	for _, pkg := range m.Packages {
		if pkg.Required && strings.EqualFold(pkg.Target, target) {
			out = append(out, pkg)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no required packages for target %s", target)
	}
	return out, nil
}

func (m Manifest) ResourceCompatible(version string) bool {
	actual, ok := parseDottedVersion(version)
	if !ok {
		return false
	}
	floor, ok := parseDottedVersion(m.MinimumResourceVersion)
	if !ok {
		return false
	}
	return compareDottedVersion(actual, floor) >= 0
}

func parseDottedVersion(version string) ([]int, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, false
	}

	parts := strings.Split(version, ".")
	out := make([]int, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, false
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

func compareDottedVersion(a, b []int) int {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		av := 0
		if i < len(a) {
			av = a[i]
		}
		bv := 0
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}
