package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/workspace"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) UpsertManagedProfile(input launchcode.ManagedProfileUpsertInput) (*launchcode.ManagedProfileUpsertResult, error) {
	profileID := strings.TrimSpace(input.ProfileID)
	shopID := strings.TrimSpace(input.ShopID)
	platformCode := strings.TrimSpace(input.PlatformCode)
	profileName := strings.TrimSpace(input.ProfileName)
	userDataDir := strings.TrimSpace(input.UserDataDir)

	if profileID == "" || shopID == "" || platformCode == "" || profileName == "" || userDataDir == "" {
		return nil, fmt.Errorf("invalid managed profile input")
	}
	if !input.ManagedMode {
		return nil, fmt.Errorf("managed mode required")
	}

	a.browserMgr.InitData()
	now := time.Now().Format(time.RFC3339)
	updated := false

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	if exists && profile != nil {
		updated = true
		profile.ProfileName = profileName
		profile.UserDataDir = userDataDir
		profile.Tags = mergeManagedTags(profile.Tags, platformCode, shopID)
		profile.UpdatedAt = now
	} else {
		coreID := ""
		if defaultCore, ok := a.browserMgr.GetDefaultCore(); ok {
			coreID = defaultCore.CoreId
		}
		a.browserMgr.Profiles[profileID] = &browser.Profile{
			ProfileId:       profileID,
			ProfileName:     profileName,
			UserDataDir:     userDataDir,
			CoreId:          coreID,
			FingerprintArgs: []string{},
			LaunchArgs:      []string{},
			Tags:            mergeManagedTags(nil, platformCode, shopID),
			Keywords:        []string{},
			Running:         false,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		profile = a.browserMgr.Profiles[profileID]
	}
	a.browserMgr.Mutex.Unlock()

	if a.browserMgr.ProfileDAO != nil {
		if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
			return nil, err
		}
	} else if err := a.browserMgr.SaveProfiles(); err != nil {
		return nil, err
	}

	return &launchcode.ManagedProfileUpsertResult{
		ProfileID: profileID,
		Updated:   updated,
	}, nil
}

func (a *App) StopInstance(profileID string) (bool, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return false, fmt.Errorf("profile id is required")
	}
	if _, err := a.BrowserInstanceStop(profileID); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "profile not found") {
			return false, err
		}
		return false, err
	}
	return true, nil
}

func (a *App) ClearProfileSession(profileID string, clearCookies bool, clearStorage bool) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return fmt.Errorf("profile id is required")
	}

	var userDataDir string
	var running bool

	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	if !exists || profile == nil {
		a.browserMgr.Mutex.Unlock()
		return fmt.Errorf("profile not found")
	}
	userDataDir = strings.TrimSpace(profile.UserDataDir)
	running = profile.Running
	a.browserMgr.Mutex.Unlock()

	if clearCookies && running {
		if err := a.BrowserClearCookies(profileID); err != nil {
			return err
		}
	}

	if clearStorage {
		if running {
			if _, err := a.BrowserInstanceStop(profileID); err != nil {
				return err
			}
		}
		root := a.browserMgr.ResolveRelativePath(userDataDir)
		if err := clearManagedStorage(root); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) InjectManagedSessionBundle(profileID string, bundle workspace.SessionBundle) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return fmt.Errorf("profile id is required")
	}
	if len(bundle.Cookies) == 0 {
		return nil
	}

	if _, err := a.BrowserInstanceStartWithParams(profileID, nil, []string{"about:blank"}, true); err != nil {
		return err
	}

	return a.browserImportCookies(profileID, bundle.Cookies)
}

func mergeManagedTags(existing []string, platformCode, shopID string) []string {
	seen := make(map[string]struct{})
	tags := make([]string, 0, len(existing)+4)
	for _, tag := range existing {
		t := strings.TrimSpace(tag)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}

	addTag := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	addTag("managed")
	addTag("managed:desktop")
	addTag("platform:" + platformCode)
	addTag("shop:" + shopID)
	return tags
}

func clearManagedStorage(userDataRoot string) error {
	userDataRoot = strings.TrimSpace(userDataRoot)
	if userDataRoot == "" {
		return nil
	}

	targets := []string{
		filepath.Join(userDataRoot, "Default", "Local Storage"),
		filepath.Join(userDataRoot, "Default", "Session Storage"),
		filepath.Join(userDataRoot, "Default", "IndexedDB"),
		filepath.Join(userDataRoot, "Default", "Service Worker"),
		filepath.Join(userDataRoot, "Default", "WebStorage"),
		filepath.Join(userDataRoot, "Default", "Code Cache"),
		filepath.Join(userDataRoot, "Default", "Cache"),
	}

	for _, target := range targets {
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return nil
}
