package launchcode

import (
	"ant-chrome/backend/internal/workspace"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ManagedProfileUpsertInput struct {
	ProfileID    string `json:"profileId"`
	ShopID       string `json:"shopId"`
	PlatformCode string `json:"platformCode"`
	ProfileName  string `json:"profileName"`
	ManagedMode  bool   `json:"managedMode"`
	UserDataDir  string `json:"userDataDir"`
}

type managedRuntimeOperator interface {
	UpsertManagedProfile(input ManagedProfileUpsertInput) (*ManagedProfileUpsertResult, error)
	StopInstance(profileID string) (bool, error)
	ClearProfileSession(profileID string, clearCookies bool, clearStorage bool) error
	InjectManagedSessionBundle(profileID string, bundle workspace.SessionBundle) error
}

type managedSessionBundleCapturer interface {
	CaptureManagedSessionBundle(profileID string, platformCode string, captureStartedAt string) (workspace.SessionBundle, error)
}

type ManagedProfileUpsertResult struct {
	ProfileID string
	Updated   bool
}

type localUpsertRequest struct {
	ProfileID    string `json:"profileId"`
	ShopID       string `json:"shopId"`
	PlatformCode string `json:"platformCode"`
	ProfileName  string `json:"profileName"`
	ManagedMode  bool   `json:"managedMode"`
	UserDataDir  string `json:"userDataDir"`
}

type localLaunchRequest struct {
	Headless      bool                    `json:"headless"`
	TargetURL     string                  `json:"targetUrl"`
	SessionBundle workspace.SessionBundle `json:"sessionBundle"`
}

type localClearSessionRequest struct {
	ClearCookies bool `json:"clearCookies"`
	ClearStorage bool `json:"clearStorage"`
}

type localInjectSessionRequest struct {
	SessionBundle workspace.SessionBundle `json:"sessionBundle"`
}

type localSessionBundleRequest struct {
	PlatformCode     string `json:"platformCode"`
	CaptureStartedAt string `json:"captureStartedAt"`
}

func (s *LaunchServer) handleLocalHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"managedMode": true,
	})
}

func (s *LaunchServer) handleLocalProfileUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	controller, ok := s.starter.(managedRuntimeOperator)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":      false,
			"message": "managed mode is unavailable",
		})
		return
	}

	payload, err := decodeLocalUpsertRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	result, err := controller.UpsertManagedProfile(ManagedProfileUpsertInput{
		ProfileID:    payload.ProfileID,
		ShopID:       payload.ShopID,
		PlatformCode: payload.PlatformCode,
		ProfileName:  payload.ProfileName,
		ManagedMode:  payload.ManagedMode,
		UserDataDir:  payload.UserDataDir,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": result.ProfileID,
		"updated":   result.Updated,
	})
}

func (s *LaunchServer) handleLocalProfileByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/local/profiles/")
	path = strings.TrimSpace(path)
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok":      false,
			"message": "not found",
		})
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok":      false,
			"message": "not found",
		})
		return
	}

	profileID := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	if profileID == "" || action == "" {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok":      false,
			"message": "not found",
		})
		return
	}

	switch action {
	case "launch":
		s.handleLocalProfileLaunch(w, r, profileID)
	case "runtime":
		s.handleLocalProfileRuntime(w, r, profileID)
	case "close":
		s.handleLocalProfileClose(w, r, profileID)
	case "clear-session":
		s.handleLocalProfileClearSession(w, r, profileID)
	case "inject-session-bundle":
		s.handleLocalProfileInjectSessionBundle(w, r, profileID)
	case "session-bundle":
		s.handleLocalProfileSessionBundle(w, r, profileID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok":      false,
			"message": "not found",
		})
	}
}

func (s *LaunchServer) handleLocalProfileLaunch(w http.ResponseWriter, r *http.Request, profileID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}
	payload, err := decodeLocalLaunchRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	if controller, ok := s.starter.(managedRuntimeOperator); ok {
		if err := controller.InjectManagedSessionBundle(profileID, payload.SessionBundle); err != nil {
			writeJSON(w, mapLaunchErrorStatus(err), map[string]interface{}{
				"ok":      false,
				"message": err.Error(),
			})
			return
		}
	}

	params := LaunchRequestParams{}
	if targetURL := strings.TrimSpace(payload.TargetURL); targetURL != "" {
		params.StartURLs = []string{targetURL}
		params.SkipDefaultStartURLs = true
	}

	profile, err := s.launchProfile(profileID, params)
	if err != nil {
		status := mapLaunchErrorStatus(err)
		writeJSON(w, status, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	s.SetActiveProfile(profile)

	currentURL := strings.TrimSpace(payload.TargetURL)
	pageTitle := ""
	if snapshot, ok := s.localLaunchSnapshot(profileID, payload); ok {
		if snapshot.CurrentURL != "" {
			currentURL = snapshot.CurrentURL
		}
		pageTitle = snapshot.PageTitle
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"profileId":  profileID,
		"pid":        profile.Pid,
		"debugPort":  profile.DebugPort,
		"targetUrl":  strings.TrimSpace(payload.TargetURL),
		"currentUrl": currentURL,
		"pageTitle":  pageTitle,
	})
}

func (s *LaunchServer) handleLocalProfileRuntime(w http.ResponseWriter, r *http.Request, profileID string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	profile, status, errMsg := s.profileSnapshotByID(profileID)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":      false,
			"message": errMsg,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"profileId":   profileID,
		"running":     profile.Running,
		"pid":         profile.Pid,
		"debugPort":   profile.DebugPort,
		"lastStartAt": profile.LastStartAt,
	})
}

func (s *LaunchServer) handleLocalProfileClose(w http.ResponseWriter, r *http.Request, profileID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	controller, ok := s.starter.(managedRuntimeOperator)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":      false,
			"message": "managed mode is unavailable",
		})
		return
	}

	closed, err := controller.StopInstance(profileID)
	if err != nil {
		writeJSON(w, mapLaunchErrorStatus(err), map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"closed":    closed,
	})
}

func (s *LaunchServer) handleLocalProfileClearSession(w http.ResponseWriter, r *http.Request, profileID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	controller, ok := s.starter.(managedRuntimeOperator)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":      false,
			"message": "managed mode is unavailable",
		})
		return
	}

	payload, err := decodeLocalClearSessionRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	if err := controller.ClearProfileSession(profileID, payload.ClearCookies, payload.ClearStorage); err != nil {
		writeJSON(w, mapLaunchErrorStatus(err), map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"cleared":   true,
	})
}

func (s *LaunchServer) handleLocalProfileInjectSessionBundle(w http.ResponseWriter, r *http.Request, profileID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	controller, ok := s.starter.(managedRuntimeOperator)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":      false,
			"message": "managed mode is unavailable",
		})
		return
	}

	payload, err := decodeLocalInjectSessionRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	if err := controller.InjectManagedSessionBundle(profileID, payload.SessionBundle); err != nil {
		writeJSON(w, mapLaunchErrorStatus(err), map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"injected":  true,
	})
}

func (s *LaunchServer) handleLocalProfileSessionBundle(w http.ResponseWriter, r *http.Request, profileID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	controller, ok := s.starter.(managedSessionBundleCapturer)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":      false,
			"message": "managed mode is unavailable",
		})
		return
	}

	payload, err := decodeLocalSessionBundleRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	bundle, err := controller.CaptureManagedSessionBundle(profileID, payload.PlatformCode, payload.CaptureStartedAt)
	if err != nil {
		writeJSON(w, mapLaunchErrorStatus(err), map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"profileId":     profileID,
		"sessionBundle": bundle,
	})
}

func decodeLocalUpsertRequest(r *http.Request) (*localUpsertRequest, error) {
	var req localUpsertRequest
	if err := decodeLocalRequestBody(r, &req); err != nil {
		return nil, err
	}
	req.ProfileID = strings.TrimSpace(req.ProfileID)
	req.ShopID = strings.TrimSpace(req.ShopID)
	req.PlatformCode = strings.TrimSpace(req.PlatformCode)
	req.ProfileName = strings.TrimSpace(req.ProfileName)
	req.UserDataDir = strings.TrimSpace(req.UserDataDir)

	if req.ProfileID == "" {
		return nil, errInvalidField("profileId")
	}
	if req.ShopID == "" {
		return nil, errInvalidField("shopId")
	}
	if req.PlatformCode == "" {
		return nil, errInvalidField("platformCode")
	}
	if req.ProfileName == "" {
		return nil, errInvalidField("profileName")
	}
	if req.UserDataDir == "" {
		return nil, errInvalidField("userDataDir")
	}
	if !req.ManagedMode {
		return nil, errInvalidField("managedMode")
	}
	return &req, nil
}

func decodeLocalLaunchRequest(r *http.Request) (*localLaunchRequest, error) {
	var req localLaunchRequest
	if err := decodeLocalRequestBody(r, &req); err != nil {
		return nil, err
	}
	req.TargetURL = strings.TrimSpace(req.TargetURL)
	return &req, nil
}

func decodeLocalClearSessionRequest(r *http.Request) (*localClearSessionRequest, error) {
	var req localClearSessionRequest
	if err := decodeLocalRequestBody(r, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func decodeLocalInjectSessionRequest(r *http.Request) (*localInjectSessionRequest, error) {
	var req localInjectSessionRequest
	if err := decodeLocalRequestBody(r, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func decodeLocalSessionBundleRequest(r *http.Request) (*localSessionBundleRequest, error) {
	var req localSessionBundleRequest
	if err := decodeLocalRequestBody(r, &req); err != nil {
		return nil, err
	}
	req.PlatformCode = strings.TrimSpace(req.PlatformCode)
	req.CaptureStartedAt = strings.TrimSpace(req.CaptureStartedAt)
	return &req, nil
}

func decodeLocalRequestBody(r *http.Request, out interface{}) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func errInvalidField(field string) error {
	return &localValidationError{message: "invalid " + field}
}

type localRuntimeTarget struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (s *LaunchServer) localLaunchSnapshot(profileID string, payload *localLaunchRequest) (workspace.OpenRuntimeSnapshot, bool) {
	profile, _, errMsg := s.profileSnapshotByID(profileID)
	if errMsg != "" || profile == nil || profile.DebugPort <= 0 {
		return workspace.OpenRuntimeSnapshot{}, false
	}

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", profile.DebugPort), nil)
	if err != nil {
		return workspace.OpenRuntimeSnapshot{}, false
	}

	resp, err := client.Do(req)
	if err != nil {
		return workspace.OpenRuntimeSnapshot{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return workspace.OpenRuntimeSnapshot{}, false
	}

	var rawTargets []localRuntimeTarget
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rawTargets); err != nil {
		return workspace.OpenRuntimeSnapshot{}, false
	}

	targets := make([]workspace.OpenRuntimeTarget, 0, len(rawTargets))
	for _, target := range rawTargets {
		if strings.TrimSpace(target.Type) != "page" {
			continue
		}
		targets = append(targets, workspace.OpenRuntimeTarget{
			CurrentURL: strings.TrimSpace(target.URL),
			PageTitle:  strings.TrimSpace(target.Title),
		})
	}
	if len(targets) == 0 {
		return workspace.OpenRuntimeSnapshot{}, false
	}

	launchContext := workspace.ShopLaunchContext{
		TargetURL: strings.TrimSpace(payload.TargetURL),
		SuccessURLPatterns: []string{
			"https://work.1688.com/",
			"https://trade.1688.com/",
			"https://air.1688.com/",
			"https://seller.1688.com/",
		},
		LoginURLPatterns: []string{
			"https://login.1688.com/",
			"https://login.taobao.com/",
			"https://login.alibaba.com/",
		},
	}
	snapshots := make([]workspace.OpenRuntimeSnapshot, 0, len(targets))
	for _, target := range targets {
		snapshots = append(snapshots, workspace.OpenRuntimeSnapshot{
			CurrentURL: target.CurrentURL,
			PageTitle:  target.PageTitle,
		})
	}

	snapshot := workspace.SelectPreferredOpenSnapshotForLaunchContext("", launchContext, snapshots)
	return snapshot, strings.TrimSpace(snapshot.CurrentURL) != "" || strings.TrimSpace(snapshot.PageTitle) != ""
}

type localValidationError struct {
	message string
}

func (e *localValidationError) Error() string {
	return e.message
}

func mapLaunchErrorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "profile not found"), strings.Contains(msg, "实例不存在"):
		return http.StatusNotFound
	case strings.Contains(msg, "service unavailable"):
		return http.StatusServiceUnavailable
	case strings.Contains(msg, "not ready"), strings.Contains(msg, "调试接口尚未就绪"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
