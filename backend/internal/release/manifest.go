package release

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Manifest struct {
	SchemaVersion          int              `json:"schemaVersion"`
	AppVersion             string           `json:"appVersion"`
	MinimumResourceVersion string           `json:"minimumResourceVersion"`
	Packages               []RuntimePackage `json:"packages"`
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
	return strings.TrimSpace(version) >= strings.TrimSpace(m.MinimumResourceVersion)
}
