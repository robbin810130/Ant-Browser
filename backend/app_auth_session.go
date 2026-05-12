package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/authsession"
	"context"
	"errors"
	"fmt"
	"net/http"
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
		return "", err
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
		return nil, err
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

func (a *App) BootstrapDesktopAuthRuntime() error {
	a.ensureWorkspaceAgentBootstrapped()
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
