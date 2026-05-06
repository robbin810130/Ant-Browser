package launchcode

import (
	"ant-chrome/backend/internal/workspace"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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
	Headless bool `json:"headless"`
}

type localClearSessionRequest struct {
	ClearCookies bool `json:"clearCookies"`
	ClearStorage bool `json:"clearStorage"`
}

type localInjectSessionRequest struct {
	SessionBundle workspace.SessionBundle `json:"sessionBundle"`
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
	if _, err := decodeLocalLaunchRequest(r); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	profile, err := s.launchProfile(profileID, LaunchRequestParams{})
	if err != nil {
		status := mapLaunchErrorStatus(err)
		writeJSON(w, status, map[string]interface{}{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	s.SetActiveProfile(profile)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"pid":       profile.Pid,
		"debugPort": profile.DebugPort,
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
