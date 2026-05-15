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
	Code              string `json:"code"`
	Severity          string `json:"severity"`
	Message           string `json:"message"`
	Repairable        bool   `json:"repairable"`
	RecommendedAction string `json:"recommendedAction,omitempty"`
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
		return blockedResult("ENV-MANIFEST-MISSING", "未找到运行时 manifest", "请确认安装目录完整，必要时重新安装应用并导出诊断包。")
	}
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return blockedResult("ENV-MANIFEST-MISSING", "未找到运行时 manifest", "请确认安装目录完整，必要时重新安装应用并导出诊断包。")
		}
		return blockedResult("ENV-MANIFEST-UNREADABLE", "运行时 manifest 不可读取", "请检查 publish/runtime-manifest.json 是否存在且当前用户有读取权限。")
	}

	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = DefaultTarget()
	}

	requiredPackages, err := c.Manifest.RequiredPackages(target)
	if err != nil {
		return blockedResult("PKG-TARGET-MISSING", fmt.Sprintf("当前平台 %s 缺少必需运行时包或未受支持", target), "请确认当前安装包包含本平台运行时资源，必要时更换正确的安装包。")
	}
	if !c.Manifest.ResourceCompatible(input.ResourceVersion) {
		return repairableResult("PKG-RESOURCE-OUTDATED", "资源版本过旧，需要修复", "请先尝试自动修复；若仍失败，请导出诊断包并核对 runtime 资源版本。")
	}

	for _, pkg := range requiredPackages {
		if !pkg.Required {
			continue
		}
		if !strings.EqualFold(pkg.Kind, "browser-core") {
			continue
		}
		if result := browser.ValidateCoreDirectory(input.BrowserCorePath); !result.Valid {
			return repairableResult("PKG-BROWSER-CORE-MISSING", result.Message, "请补齐或重新下载浏览器内核后再试。")
		}
	}

	return CheckResult{State: StatePass}
}

func blockedResult(code, message, recommendedAction string) CheckResult {
	return CheckResult{State: StateBlocked, Items: []FailureItem{{
		Code:              code,
		Severity:          "error",
		Message:           message,
		Repairable:        false,
		RecommendedAction: strings.TrimSpace(recommendedAction),
	}}}
}

func repairableResult(code, message, recommendedAction string) CheckResult {
	return CheckResult{State: StateRepairable, Items: []FailureItem{{
		Code:              code,
		Severity:          "error",
		Message:           message,
		Repairable:        true,
		RecommendedAction: strings.TrimSpace(recommendedAction),
	}}}
}

func ResolvePackagePath(versionDir string, pkg RuntimePackage) string {
	versionDir = strings.TrimSpace(versionDir)
	packagePath := strings.TrimSpace(pkg.Path)
	if versionDir == "" || packagePath == "" {
		return ""
	}
	if filepath.IsAbs(packagePath) {
		return ""
	}
	joined := filepath.Join(versionDir, filepath.FromSlash(packagePath))
	rel, err := filepath.Rel(versionDir, joined)
	if err != nil {
		return ""
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return joined
}
