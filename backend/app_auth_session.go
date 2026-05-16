package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/authsession"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type DesktopAuthSession = authsession.Session

type DesktopAuthUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
}

type DesktopAuthRole struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type DesktopAuthProfile struct {
	User      DesktopAuthUser   `json:"user"`
	Roles     []DesktopAuthRole `json:"roles"`
	DataScope string            `json:"dataScope"`
}

type desktopAuthProfileEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		User      DesktopAuthUser   `json:"user"`
		Roles     []DesktopAuthRole `json:"roles"`
		DataScope string            `json:"dataScope"`
	} `json:"data"`
}

type desktopAuthLoginEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken string `json:"accessToken"`
	} `json:"data"`
}

type DesktopSharedLoginBindSession struct {
	BindSessionID        string `json:"bindSessionId"`
	TraceID              string `json:"traceId"`
	ShopID               string `json:"shopId"`
	ShopName             string `json:"shopName"`
	SessionType          string `json:"sessionType"`
	Status               string `json:"status"`
	StatusLabel          string `json:"statusLabel"`
	Message              string `json:"message"`
	ManualActionRequired bool   `json:"manualActionRequired"`
	LastObservedURL      string `json:"lastObservedUrl"`
	StartedAt            string `json:"startedAt"`
	ExpiresAt            string `json:"expiresAt"`
	CompletedAt          string `json:"completedAt"`
	UpdatedAt            string `json:"updatedAt"`
	ChallengeType        string `json:"challengeType"`
}

type DesktopSharedLoginDetail struct {
	ShopID                 string `json:"shopId"`
	ShopName               string `json:"shopName"`
	PlatformCode           string `json:"platformCode"`
	SharedLoginStatus      string `json:"sharedLoginStatus"`
	SharedLoginStatusLabel string `json:"sharedLoginStatusLabel"`
}

type DesktopSharedLoginActionResult struct {
	BindSession DesktopSharedLoginBindSession `json:"bindSession"`
	Detail      DesktopSharedLoginDetail      `json:"detail"`
}

type desktopSharedLoginActionEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		BindSession DesktopSharedLoginBindSession `json:"bindSession"`
		Detail      struct {
			ShopID       string `json:"shopId"`
			ShopName     string `json:"shopName"`
			PlatformCode string `json:"platformCode"`
			SharedLogin  struct {
				Status      string `json:"status"`
				StatusLabel string `json:"statusLabel"`
			} `json:"sharedLogin"`
		} `json:"detail"`
	} `json:"data"`
}

type desktopSharedLoginBindSessionEnvelope struct {
	Code    int                           `json:"code"`
	Message string                        `json:"message"`
	Data    DesktopSharedLoginBindSession `json:"data"`
}

func (a *App) LoadDesktopAuthSession() (*DesktopAuthSession, error) {
	session, err := a.desktopAuthSessionStore().Load()
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (a *App) SaveDesktopAuthSession(accessToken string, rememberMe bool) error {
	return a.desktopAuthSessionStore().Save(authsession.Session{
		AccessToken: strings.TrimSpace(accessToken),
		RememberMe:  rememberMe,
	})
}

func (a *App) LoginDesktopUser(username, password string) (string, error) {
	serverOrigin := resolveWorkspaceServerOriginWithConfig(resolveWorkspaceRuntimeDirWithConfig(a.config), a.config)
	var envelope desktopAuthLoginEnvelope
	if err := postWorkspaceJSON(strings.TrimRight(serverOrigin, "/")+"/api/auth/login", map[string]string{
		"username": strings.TrimSpace(username),
		"password": strings.TrimSpace(password),
	}, &envelope); err != nil {
		return "", normalizeDesktopWorkspaceRequestError(err, serverOrigin)
	}

	accessToken := strings.TrimSpace(envelope.Data.AccessToken)
	if accessToken == "" {
		return "", fmt.Errorf("desktop auth login access token is required")
	}
	return accessToken, nil
}

func (a *App) FetchDesktopAuthProfile(accessToken string) (*DesktopAuthProfile, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("desktop auth access token is required")
	}

	serverOrigin := resolveWorkspaceServerOriginWithConfig(resolveWorkspaceRuntimeDirWithConfig(a.config), a.config)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, strings.TrimRight(serverOrigin, "/")+"/api/auth/me", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("accept", "application/json")
	request.Header.Set("authorization", "Bearer "+accessToken)

	var envelope desktopAuthProfileEnvelope
	if err := doWorkspaceJSON(request, &envelope); err != nil {
		return nil, normalizeDesktopWorkspaceRequestError(err, serverOrigin)
	}
	if strings.TrimSpace(envelope.Data.User.ID) == "" {
		return nil, fmt.Errorf("desktop auth profile user id is required")
	}

	return &DesktopAuthProfile{
		User:      envelope.Data.User,
		Roles:     append([]DesktopAuthRole(nil), envelope.Data.Roles...),
		DataScope: strings.TrimSpace(envelope.Data.DataScope),
	}, nil
}

func (a *App) ClearDesktopAuthSession() error {
	return a.desktopAuthSessionStore().Save(authsession.Session{})
}

func (a *App) StartDesktopSharedLoginBind(accessToken, shopID string) (*DesktopSharedLoginActionResult, error) {
	return a.startDesktopSharedLoginAction(accessToken, shopID, "/api/desktop/shops/%s/bind")
}

func (a *App) StartDesktopSharedLoginValidate(accessToken, shopID string) (*DesktopSharedLoginActionResult, error) {
	return a.startDesktopSharedLoginAction(accessToken, shopID, "/api/desktop/shops/%s/validate")
}

func (a *App) FetchDesktopSharedLoginBindSession(accessToken, bindSessionID string) (*DesktopSharedLoginBindSession, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("desktop auth access token is required")
	}

	bindSessionID = strings.TrimSpace(bindSessionID)
	if bindSessionID == "" {
		return nil, fmt.Errorf("bind session id is required")
	}

	var envelope desktopSharedLoginBindSessionEnvelope
	if err := a.doDesktopAuthedWorkspaceJSON(accessToken, http.MethodGet, fmt.Sprintf("/api/desktop/shared-login-bind-sessions/%s", bindSessionID), nil, &envelope); err != nil {
		return nil, err
	}

	session := envelope.Data
	session.BindSessionID = strings.TrimSpace(session.BindSessionID)
	if session.BindSessionID == "" {
		return nil, fmt.Errorf("desktop shared login bind session id is required")
	}
	return &session, nil
}

func (a *App) BootstrapDesktopAuthRuntime() error {
	if err := a.ensureWorkspaceAgentBootstrapped(); err != nil {
		return err
	}
	_, err := a.WorkspaceAuthorizedShops()
	return err
}

func (a *App) DesktopAuthStrongCleanup(reason string) error {
	var cleanupErrs []error

	for _, profileID := range a.desktopAuthCleanupProfileIDs() {
		if _, err := a.StopInstance(profileID); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}

	if err := a.resetWorkspaceAgentSessionRuntimeHook(reason); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}
	if err := a.ClearDesktopAuthSession(); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}

	return errors.Join(cleanupErrs...)
}

func (a *App) desktopAuthSessionStore() *authsession.Store {
	if a == nil {
		return authsession.NewStore("")
	}
	if a.authSessionStore == nil {
		a.authSessionStore = authsession.NewStore(apppath.StateRoot(a.appRoot))
	}
	return a.authSessionStore
}

func (a *App) desktopAuthCleanupProfileIDs() []string {
	if a == nil || a.browserMgr == nil {
		return nil
	}

	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profileIDs := make([]string, 0)
	for profileID, profile := range a.browserMgr.Profiles {
		if profile == nil || !profile.Running || !isDesktopManagedProfile(profile.Tags) {
			continue
		}
		profileIDs = append(profileIDs, profileID)
	}
	sort.Strings(profileIDs)
	return profileIDs
}

func isDesktopManagedProfile(tags []string) bool {
	return hasDesktopManagedTag(tags, "managed") && hasDesktopManagedTag(tags, "managed:desktop")
}

func hasDesktopManagedTag(tags []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), target) {
			return true
		}
	}
	return false
}

func (a *App) startDesktopSharedLoginAction(accessToken, shopID, pathTemplate string) (*DesktopSharedLoginActionResult, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("desktop auth access token is required")
	}

	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return nil, fmt.Errorf("shop id is required")
	}

	var envelope desktopSharedLoginActionEnvelope
	if err := a.doDesktopAuthedWorkspaceJSON(accessToken, http.MethodPost, fmt.Sprintf(pathTemplate, shopID), map[string]any{}, &envelope); err != nil {
		return nil, err
	}

	result := &DesktopSharedLoginActionResult{
		BindSession: envelope.Data.BindSession,
		Detail: DesktopSharedLoginDetail{
			ShopID:                 strings.TrimSpace(envelope.Data.Detail.ShopID),
			ShopName:               strings.TrimSpace(envelope.Data.Detail.ShopName),
			PlatformCode:           strings.TrimSpace(envelope.Data.Detail.PlatformCode),
			SharedLoginStatus:      strings.TrimSpace(envelope.Data.Detail.SharedLogin.Status),
			SharedLoginStatusLabel: strings.TrimSpace(envelope.Data.Detail.SharedLogin.StatusLabel),
		},
	}
	result.BindSession.BindSessionID = strings.TrimSpace(result.BindSession.BindSessionID)
	if result.BindSession.BindSessionID == "" {
		return nil, fmt.Errorf("desktop shared login bind session id is required")
	}
	return result, nil
}

func (a *App) doDesktopAuthedWorkspaceJSON(accessToken, method, path string, body any, dest any) error {
	serverOrigin := resolveWorkspaceServerOriginWithConfig(resolveWorkspaceRuntimeDirWithConfig(a.config), a.config)
	serverOrigin = strings.TrimRight(strings.TrimSpace(serverOrigin), "/")
	if serverOrigin == "" {
		return fmt.Errorf("workspace server origin is required")
	}

	var requestBody any
	if method != http.MethodGet && body == nil {
		requestBody = map[string]any{}
	} else {
		requestBody = body
	}

	url := serverOrigin + path
	var request *http.Request
	var err error
	if method == http.MethodGet {
		request, err = http.NewRequestWithContext(context.Background(), method, url, nil)
	} else {
		payload, marshalErr := jsonMarshalWorkspaceBody(requestBody)
		if marshalErr != nil {
			return marshalErr
		}
		request, err = http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(payload))
		if err == nil {
			request.Header.Set("content-type", "application/json")
		}
	}
	if err != nil {
		return err
	}
	request.Header.Set("accept", "application/json")
	request.Header.Set("authorization", "Bearer "+accessToken)

	return normalizeDesktopWorkspaceRequestError(doWorkspaceJSON(request, dest), serverOrigin)
}

func jsonMarshalWorkspaceBody(body any) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func normalizeDesktopWorkspaceRequestError(err error, serverOrigin string) error {
	if err == nil {
		return nil
	}
	if !isDesktopWorkspaceConnectionError(err) {
		return err
	}

	origin := strings.TrimRight(strings.TrimSpace(serverOrigin), "/")
	if origin == "" {
		return fmt.Errorf("workspace 服务端不可达: %w", err)
	}
	return fmt.Errorf("workspace 服务端不可达，请检查服务是否已启动 (%s): %w", origin, err)
}

func isDesktopWorkspaceConnectionError(err error) bool {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return false
	}
	if urlErr.Timeout() {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}
