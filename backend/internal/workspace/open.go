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
		"工作台",
	}
)

type OpenRuntimeSnapshot struct {
	CurrentURL string `json:"currentUrl"`
	PageTitle  string `json:"pageTitle"`
}

type SessionCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HttpOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
	URL      string  `json:"url"`
}

type SessionStorageEntry struct {
	Origin string            `json:"origin"`
	Scope  string            `json:"scope"`
	Items  map[string]string `json:"items"`
}

type SessionBundle struct {
	UserAgent       string                `json:"userAgent"`
	Cookies         []SessionCookie       `json:"cookies"`
	Storages        []SessionStorageEntry `json:"storages"`
	LastObservedURL string                `json:"lastObservedUrl"`
}

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

	if matchesAnyPrefix(lowerURL, defaultSuccessURLPatterns) && containsAny(lowerTitle, defaultBackendTitleKeywords) {
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
