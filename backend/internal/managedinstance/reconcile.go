package managedinstance

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/workspace"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const reclaimPendingTag = "managed:reclaim-pending"

func (s *Service) ReconcileAuthorizedShops(shops []workspace.ShopRecord) (*workspace.ReconcileSummary, error) {
	if s == nil || s.browserMgr == nil {
		return nil, fmt.Errorf("managed instance service is not ready")
	}

	result := &workspace.ReconcileSummary{}
	authorized := make(map[string]workspace.ShopRecord, len(shops))
	for _, shop := range shops {
		profileID := workspace.BuildProfileID(shop.PlatformCode, shop.ShopID)
		authorized[profileID] = shop

		existed := s.managedProfileExists(profileID)
		if err := s.upsertAuthorizedProfile(profileID, shop); err != nil {
			return nil, err
		}
		if existed {
			result.UpdatedProfileIDs = appendIfMissing(result.UpdatedProfileIDs, profileID)
		} else {
			result.CreatedProfileIDs = appendIfMissing(result.CreatedProfileIDs, profileID)
		}
	}

	for _, profile := range s.browserMgr.ListByTag("managed") {
		if !isManagedDesktopProfile(profile) {
			continue
		}
		if _, ok := authorized[strings.TrimSpace(profile.ProfileId)]; ok {
			continue
		}
		if err := s.reclaimManagedProfile(profile); err != nil {
			if markErr := s.setReclaimPending(profile.ProfileId, true); markErr != nil {
				return nil, markErr
			}
			result.PendingReclaimProfileIDs = appendIfMissing(result.PendingReclaimProfileIDs, profile.ProfileId)
			continue
		}
		result.ReclaimedProfileIDs = appendIfMissing(result.ReclaimedProfileIDs, profile.ProfileId)
	}

	return result, nil
}

func (s *Service) managedProfileExists(profileID string) bool {
	s.browserMgr.InitData()
	s.browserMgr.Mutex.Lock()
	defer s.browserMgr.Mutex.Unlock()
	profile, exists := s.browserMgr.Profiles[strings.TrimSpace(profileID)]
	return exists && profile != nil
}

func (s *Service) upsertAuthorizedProfile(profileID string, shop workspace.ShopRecord) error {
	platformCode := strings.TrimSpace(shop.PlatformCode)
	if platformCode == "" {
		platformCode = platformCodeFromProfileID(profileID)
	}
	now := time.Now().Format(time.RFC3339)

	s.browserMgr.InitData()
	s.browserMgr.Mutex.Lock()
	profile, exists := s.browserMgr.Profiles[profileID]
	if exists && profile != nil {
		profile.ProfileName = firstNonEmptyString(strings.TrimSpace(shop.ShopName), profile.ProfileName, profileID)
		profile.UserDataDir = firstNonEmptyString(strings.TrimSpace(profile.UserDataDir), managedProfileUserDataDir(profileID))
		profile.Tags = clearManagedTag(mergeManagedTags(profile.Tags, platformCode, shop.ShopID), reclaimPendingTag)
		profile.UpdatedAt = now
		if strings.TrimSpace(profile.CoreId) == "" {
			if defaultCore, ok := s.browserMgr.GetDefaultCore(); ok {
				profile.CoreId = defaultCore.CoreId
			}
		}
		snapshot := *profile
		s.browserMgr.Mutex.Unlock()
		return s.persistProfile(&snapshot)
	}

	newProfile := &browser.Profile{
		ProfileId:       profileID,
		ProfileName:     firstNonEmptyString(strings.TrimSpace(shop.ShopName), profileID),
		UserDataDir:     managedProfileUserDataDir(profileID),
		FingerprintArgs: []string{},
		LaunchArgs:      []string{},
		Tags:            mergeManagedTags(nil, platformCode, shop.ShopID),
		Keywords:        []string{},
		Running:         false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if defaultCore, ok := s.browserMgr.GetDefaultCore(); ok {
		newProfile.CoreId = defaultCore.CoreId
	}
	s.browserMgr.Profiles[profileID] = newProfile
	s.browserMgr.Mutex.Unlock()
	return s.persistProfile(newProfile)
}

func (s *Service) reclaimManagedProfile(profile browser.Profile) error {
	if profile.Running {
		runtime := s.getReclaimRuntime()
		if runtime.StopManagedProfile != nil {
			if err := runtime.StopManagedProfile(profile.ProfileId); err != nil {
				return err
			}
		}
	}
	if err := s.browserMgr.Delete(profile.ProfileId); err != nil {
		return err
	}
	if userDataDir := strings.TrimSpace(profile.UserDataDir); userDataDir != "" {
		for _, path := range []string{
			s.browserMgr.ResolveUserDataDir(&browser.Profile{ProfileId: profile.ProfileId, UserDataDir: userDataDir}),
			s.browserMgr.ResolveRelativePath(userDataDir),
		} {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) getReclaimRuntime() NativeOpenRuntime {
	if s == nil {
		return NativeOpenRuntime{}
	}
	s.runtimeMu.RLock()
	runtime := s.openRuntime
	s.runtimeMu.RUnlock()
	return runtime
}

func (s *Service) setReclaimPending(profileID string, pending bool) error {
	s.browserMgr.InitData()
	s.browserMgr.Mutex.Lock()
	profile, exists := s.browserMgr.Profiles[strings.TrimSpace(profileID)]
	if !exists || profile == nil {
		s.browserMgr.Mutex.Unlock()
		return fmt.Errorf("managed profile not found: %s", profileID)
	}
	if pending {
		profile.Tags = mergeManagedTags(profile.Tags, platformCodeFromProfileID(profile.ProfileId), shopIDFromProfile(profile.Tags))
		if !hasManagedTag(profile.Tags, reclaimPendingTag) {
			profile.Tags = append(profile.Tags, reclaimPendingTag)
		}
	} else {
		profile.Tags = clearManagedTag(profile.Tags, reclaimPendingTag)
	}
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	snapshot := *profile
	s.browserMgr.Mutex.Unlock()
	return s.persistProfile(&snapshot)
}

func (s *Service) persistProfile(profile *browser.Profile) error {
	if s.browserMgr.ProfileDAO != nil {
		return s.browserMgr.ProfileDAO.Upsert(profile)
	}
	return s.browserMgr.SaveProfiles()
}

func managedProfileUserDataDir(profileID string) string {
	return filepath.Join("managed-profiles", strings.ReplaceAll(strings.TrimSpace(profileID), ":", "__"))
}

func isManagedDesktopProfile(profile browser.Profile) bool {
	return hasManagedTag(profile.Tags, "managed") && hasManagedTag(profile.Tags, "managed:desktop")
}

func hasManagedTag(tags []string, target string) bool {
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

func clearManagedTag(tags []string, target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return append([]string{}, tags...)
	}
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), target) {
			continue
		}
		cleaned = append(cleaned, tag)
	}
	return cleaned
}

func shopIDFromProfile(tags []string) string {
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "shop:") {
			return strings.TrimSpace(strings.TrimPrefix(tag, "shop:"))
		}
	}
	return ""
}

func platformCodeFromProfileID(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if idx := strings.Index(profileID, ":"); idx > 0 {
		return strings.TrimSpace(profileID[:idx])
	}
	return ""
}

func mergeManagedTags(existing []string, platformCode, shopID string) []string {
	seen := make(map[string]struct{})
	tags := make([]string, 0, len(existing)+4)
	for _, tag := range existing {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
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
	addTag("platform:" + strings.TrimSpace(platformCode))
	addTag("shop:" + strings.TrimSpace(shopID))
	return tags
}

func appendIfMissing(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return items
		}
	}
	return append(items, value)
}
