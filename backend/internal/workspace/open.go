package workspace

import "strings"

var (
	defaultSuccessURLPatterns = []string{
		"https://work.1688.com/",
		"https://work.1688.com/?",
	}
	defaultLoginURLPatterns = []string{
		"https://login.1688.com/",
		"https://login.alibaba.com/",
	}
	defaultLoginTitleKeywords = []string{
		"登录",
		"login",
	}
	defaultBackendTitleKeywords = []string{
		"1688后台",
		"后台管理",
		"商家工作台",
		"卖家工作台",
	}
	defaultChallengeURLKeywords = []string{
		"captcha",
		"verify",
		"challenge",
		"checkcode",
		"verifycenter",
	}
)

type OpenRuntimeSnapshot struct {
	CurrentURL string `json:"currentUrl"`
	PageTitle  string `json:"pageTitle"`
}

type OpenRuntimeTarget struct {
	TargetID   string `json:"targetId"`
	CurrentURL string `json:"currentUrl"`
	PageTitle  string `json:"pageTitle"`
}

type OpenTargetAction string

const (
	OpenTargetActionActivate OpenTargetAction = "activate"
	OpenTargetActionNavigate OpenTargetAction = "navigate"
	OpenTargetActionCreate   OpenTargetAction = "create"
)

type OpenShopResult struct {
	ShopID     string `json:"shopId"`
	ProfileID  string `json:"profileId"`
	InstanceID string `json:"instanceId"`
	CurrentURL string `json:"currentUrl"`
	PageTitle  string `json:"pageTitle"`
	Success    bool   `json:"success"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func ClassifyOpenResult(snapshot OpenRuntimeSnapshot) OpenShopResult {
	return ClassifyOpenResultForShop("", snapshot)
}

func ClassifyOpenResultForLaunchContext(shopID string, launchContext ShopLaunchContext, snapshot OpenRuntimeSnapshot) OpenShopResult {
	currentURL := strings.TrimSpace(snapshot.CurrentURL)
	pageTitle := strings.TrimSpace(snapshot.PageTitle)
	lowerURL := strings.ToLower(currentURL)
	lowerTitle := strings.ToLower(pageTitle)

	loginPatterns := normalizedURLPatterns(launchContext.LoginURLPatterns, defaultLoginURLPatterns)
	successPatterns := normalizedURLPatterns(launchContext.SuccessURLPatterns, defaultSuccessURLPatterns)

	if matchesAnyURLPattern(lowerURL, loginPatterns) || containsAny(lowerTitle, defaultLoginTitleKeywords) {
		return OpenShopResult{
			Code:    "ANT_BACKEND_LOGIN_REQUIRED",
			Message: "未能打开目标店铺后台，请先执行更新凭据后重试",
		}
	}

	if containsAny(lowerURL, defaultChallengeURLKeywords) {
		return OpenShopResult{
			Code:    "ANT_MANUAL_VERIFICATION_REQUIRED",
			Message: "当前店铺需要人工验证后才能继续打开后台",
		}
	}

	if matchesAnyURLPattern(lowerURL, successPatterns) {
		if !containsAny(lowerTitle, defaultBackendTitleKeywords) {
			return OpenShopResult{
				Code:    "ANT_INSTANCE_OPEN_FAILED",
				Message: "未能打开目标店铺后台，请稍后重试",
			}
		}
		if normalizedShopID := strings.ToLower(strings.TrimSpace(shopID)); normalizedShopID != "" && urlDeclaresDifferentShop(lowerURL, normalizedShopID) {
			return OpenShopResult{
				Code:    "ANT_BACKEND_TARGET_MISMATCH",
				Message: "当前实例未进入目标店铺后台，请刷新会话后重试",
			}
		}
		return OpenShopResult{
			Success: true,
		}
	}

	return OpenShopResult{
		Code:    "ANT_INSTANCE_OPEN_FAILED",
		Message: "未能打开目标店铺后台，请稍后重试",
	}
}

func ClassifyOpenResultForShop(shopID string, snapshot OpenRuntimeSnapshot) OpenShopResult {
	currentURL := strings.TrimSpace(snapshot.CurrentURL)
	pageTitle := strings.TrimSpace(snapshot.PageTitle)
	lowerURL := strings.ToLower(currentURL)
	lowerTitle := strings.ToLower(pageTitle)

	if matchesAnyPrefix(lowerURL, defaultLoginURLPatterns) || containsAny(lowerTitle, defaultLoginTitleKeywords) {
		return OpenShopResult{
			Code:    "ANT_BACKEND_LOGIN_REQUIRED",
			Message: "未能打开目标店铺后台，请先执行更新凭据后重试",
		}
	}

	if containsAny(lowerURL, defaultChallengeURLKeywords) {
		return OpenShopResult{
			Code:    "ANT_MANUAL_VERIFICATION_REQUIRED",
			Message: "当前店铺需要人工验证后才能继续打开后台",
		}
	}

	if matchesAnyPrefix(lowerURL, defaultSuccessURLPatterns) && containsAny(lowerTitle, defaultBackendTitleKeywords) {
		if normalizedShopID := strings.ToLower(strings.TrimSpace(shopID)); normalizedShopID != "" && urlDeclaresDifferentShop(lowerURL, normalizedShopID) {
			return OpenShopResult{
				Code:    "ANT_BACKEND_TARGET_MISMATCH",
				Message: "当前实例未进入目标店铺后台，请刷新会话后重试",
			}
		}
		return OpenShopResult{
			Success: true,
		}
	}

	return OpenShopResult{
		Code:    "ANT_INSTANCE_OPEN_FAILED",
		Message: "未能打开目标店铺后台，请稍后重试",
	}
}

func DefaultBackendURL(shopID string) string {
	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return "https://work.1688.com/"
	}
	return "https://work.1688.com/?shopId=" + shopID
}

func ResolveWorkspaceTargetURL(shopID string, launchTargetURL string) string {
	launchTargetURL = strings.TrimSpace(launchTargetURL)
	if launchTargetURL == "" {
		return DefaultBackendURL(shopID)
	}
	lowerURL := strings.ToLower(launchTargetURL)
	if lowerURL == "about:blank" || strings.HasPrefix(lowerURL, "chrome://newtab") || strings.HasPrefix(lowerURL, "chrome://new-tab-page") {
		return DefaultBackendURL(shopID)
	}
	return launchTargetURL
}

func SelectPreferredOpenSnapshot(shopID string, snapshots []OpenRuntimeSnapshot) OpenRuntimeSnapshot {
	normalizedShopID := strings.ToLower(strings.TrimSpace(shopID))
	if normalizedShopID == "" {
		if len(snapshots) == 0 {
			return OpenRuntimeSnapshot{}
		}
		return snapshots[0]
	}

	for _, snapshot := range snapshots {
		if strings.Contains(strings.ToLower(strings.TrimSpace(snapshot.CurrentURL)), normalizedShopID) {
			return snapshot
		}
	}
	for _, snapshot := range snapshots {
		if ClassifyOpenResultForShop("", snapshot).Code == "ANT_BACKEND_LOGIN_REQUIRED" {
			return snapshot
		}
	}
	for _, snapshot := range snapshots {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(snapshot.CurrentURL)), "https://work.1688.com/") {
			return snapshot
		}
	}
	if len(snapshots) == 0 {
		return OpenRuntimeSnapshot{}
	}
	return snapshots[0]
}

func SelectPreferredOpenSnapshotForLaunchContext(shopID string, launchContext ShopLaunchContext, snapshots []OpenRuntimeSnapshot) OpenRuntimeSnapshot {
	normalizedShopID := strings.ToLower(strings.TrimSpace(shopID))
	if normalizedShopID != "" {
		for _, snapshot := range snapshots {
			if strings.Contains(strings.ToLower(strings.TrimSpace(snapshot.CurrentURL)), normalizedShopID) {
				return snapshot
			}
		}
	}
	for _, snapshot := range snapshots {
		if ClassifyOpenResultForLaunchContext(shopID, launchContext, snapshot).Code == "ANT_BACKEND_LOGIN_REQUIRED" {
			return snapshot
		}
	}
	for _, snapshot := range snapshots {
		if candidate := ClassifyOpenResultForLaunchContext(shopID, launchContext, snapshot); candidate.Success {
			return snapshot
		}
	}
	if len(snapshots) == 0 {
		return OpenRuntimeSnapshot{}
	}
	return snapshots[0]
}

func PickOpenTargetForLaunchContext(shopID string, launchContext ShopLaunchContext, targets []OpenRuntimeTarget) (OpenRuntimeTarget, OpenTargetAction, bool) {
	for _, target := range targets {
		if ClassifyOpenResultForLaunchContext(shopID, launchContext, OpenRuntimeSnapshot{
			CurrentURL: target.CurrentURL,
			PageTitle:  target.PageTitle,
		}).Success {
			return target, OpenTargetActionActivate, true
		}
	}

	for _, target := range targets {
		if isBlankRuntimeTarget(target) {
			return target, OpenTargetActionNavigate, true
		}
	}

	return OpenRuntimeTarget{}, OpenTargetActionCreate, true
}

func CollectClosableBlankTargetIDs(targets []OpenRuntimeTarget, keepTargetID string) []string {
	closable := make([]string, 0, len(targets))
	keepTargetID = strings.TrimSpace(keepTargetID)
	for _, target := range targets {
		targetID := strings.TrimSpace(target.TargetID)
		if targetID == "" || targetID == keepTargetID {
			continue
		}
		if isBlankRuntimeTarget(target) {
			closable = append(closable, targetID)
		}
	}
	return closable
}

func urlDeclaresDifferentShop(lowerURL, expectedShopID string) bool {
	if lowerURL == "" || expectedShopID == "" {
		return false
	}
	if strings.Contains(lowerURL, expectedShopID) {
		return false
	}
	return strings.Contains(lowerURL, "shopid=")
}

func isBlankRuntimeTarget(target OpenRuntimeTarget) bool {
	lowerURL := strings.ToLower(strings.TrimSpace(target.CurrentURL))
	lowerTitle := strings.ToLower(strings.TrimSpace(target.PageTitle))
	if lowerURL == "" || lowerURL == "about:blank" || lowerURL == "chrome://newtab/" || lowerURL == "chrome://new-tab-page/" {
		return true
	}
	return strings.Contains(lowerTitle, "新标签页") || strings.Contains(lowerTitle, "new tab")
}

func normalizedURLPatterns(patterns []string, defaults []string) []string {
	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		normalized = append(normalized, pattern)
	}
	if len(normalized) > 0 {
		return normalized
	}
	return defaults
}

func matchesAnyURLPattern(value string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func matchesAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, strings.ToLower(strings.TrimSpace(prefix))) {
			return true
		}
	}
	return false
}

func containsAny(value string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(value, strings.ToLower(strings.TrimSpace(keyword))) {
			return true
		}
	}
	return false
}
