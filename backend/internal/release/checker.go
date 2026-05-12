package release

import (
	"ant-chrome/backend/internal/browser"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

type CheckState string

const (
	StatePass       CheckState = "pass"
	StateRepairable CheckState = "repairable"
	StateBlocked    CheckState = "blocked"
)

type FailureItem struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Repairable bool   `json:"repairable"`
}

type CheckInput struct {
	ManifestPath    string
	Target          string
	ResourceVersion string
	BrowserCorePath string
}

type CheckResult struct {
	State CheckState    `json:"state"`
	Items []FailureItem `json:"items"`
}

type Checker struct {
	Manifest Manifest
}

func DefaultTarget() string {
	return fmt.Sprintf("%s-%s", goruntime.GOOS, goruntime.GOARCH)
}

func (c Checker) Run(input CheckInput) CheckResult {
	manifestPath := strings.TrimSpace(input.ManifestPath)
	if manifestPath == "" {
		return blockedResult("ENV-MANIFEST-MISSING", "未找到运行时 manifest")
	}
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return blockedResult("ENV-MANIFEST-MISSING", "未找到运行时 manifest")
		}
		return blockedResult("ENV-MANIFEST-UNREADABLE", "运行时 manifest 不可读取")
	}

	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = DefaultTarget()
	}

	requiredPackages, err := c.Manifest.RequiredPackages(target)
	if err != nil {
		return repairableResult("PKG-TARGET-MISSING", fmt.Sprintf("目标平台 %s 缺少必需运行时包", target))
	}
	if !c.Manifest.ResourceCompatible(input.ResourceVersion) {
		return repairableResult("PKG-RESOURCE-OUTDATED", "资源版本过旧，需要修复")
	}

	for _, pkg := range requiredPackages {
		if !pkg.Required {
			continue
		}
		if !strings.EqualFold(pkg.Kind, "browser-core") {
			continue
		}
		if result := browser.ValidateCoreDirectory(input.BrowserCorePath); !result.Valid {
			return repairableResult("PKG-BROWSER-CORE-MISSING", result.Message)
		}
	}

	return CheckResult{State: StatePass}
}

func blockedResult(code, message string) CheckResult {
	return CheckResult{State: StateBlocked, Items: []FailureItem{{
		Code:       code,
		Severity:   "error",
		Message:    message,
		Repairable: false,
	}}}
}

func repairableResult(code, message string) CheckResult {
	return CheckResult{State: StateRepairable, Items: []FailureItem{{
		Code:       code,
		Severity:   "error",
		Message:    message,
		Repairable: true,
	}}}
}

func ResolvePackagePath(versionDir string, pkg RuntimePackage) string {
	versionDir = strings.TrimSpace(versionDir)
	packagePath := strings.TrimSpace(pkg.Path)
	if versionDir == "" || packagePath == "" {
		return ""
	}
	return filepath.Join(versionDir, filepath.FromSlash(packagePath))
}
