package workspace

import (
	"ant-chrome/backend/internal/browser"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type WorkspaceClient struct {
	baseURL    string
	httpClient *http.Client
}

type WorkspaceService struct {
	client      *WorkspaceClient
	profileList profileLister
	reconciler  authorizedShopReconciler
}

type profileLister interface {
	List() []browser.Profile
	GetCore(coreId string) (browser.Core, bool)
	GetDefaultCore() (browser.Core, bool)
}

type authorizedShopReconciler interface {
	ReconcileAuthorizedShops(shops []ShopRecord) (*ReconcileSummary, error)
}

type envelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type shopsPayload struct {
	Items []ShopRecord `json:"items"`
}

type shopProfilesPayload struct {
	Items []ShopProfileRecord `json:"items"`
}

func NewWorkspaceClient(baseURL string, client *http.Client) *WorkspaceClient {
	if client == nil {
		client = &http.Client{}
	}
	return &WorkspaceClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: client,
	}
}

func (c *WorkspaceClient) SetBaseURL(baseURL string) {
	if c == nil {
		return
	}
	c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func NewService(client *WorkspaceClient, profileList profileLister, reconciler authorizedShopReconciler) *WorkspaceService {
	return &WorkspaceService{
		client:      client,
		profileList: profileList,
		reconciler:  reconciler,
	}
}

func (s *WorkspaceService) SetBaseURL(baseURL string) {
	if s == nil || s.client == nil {
		return
	}
	s.client.SetBaseURL(baseURL)
}

func (c *WorkspaceClient) FetchWorkspaceSummary(ctx context.Context) (*WorkspaceSummary, error) {
	var payload WorkspaceSummary
	if err := c.getJSON(ctx, "/local/health", &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *WorkspaceClient) FetchAuthorizedShops(ctx context.Context) ([]ShopRecord, error) {
	var payload shopsPayload
	if err := c.getJSON(ctx, "/local/shops", &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func (c *WorkspaceClient) FetchShopProfiles(ctx context.Context) ([]ShopProfileRecord, error) {
	var payload shopProfilesPayload
	if err := c.getJSON(ctx, "/local/shop-profiles", &payload); err == nil {
		return normalizeShopProfiles(payload.Items), nil
	} else if !isWorkspaceEndpointNotFound(err) {
		return nil, err
	}

	shops, err := c.FetchAuthorizedShops(ctx)
	if err != nil {
		return nil, err
	}
	profiles := make([]ShopProfileRecord, 0, len(shops))
	for _, shop := range shops {
		profiles = append(profiles, ShopProfileRecord{
			ShopID:                   strings.TrimSpace(shop.ShopID),
			ShopName:                 strings.TrimSpace(shop.ShopName),
			PlatformCode:             strings.TrimSpace(shop.PlatformCode),
			ASMStatus:                "unavailable",
			AuthorizationStatus:      strings.TrimSpace(shop.SharedLoginStatus),
			AuthorizationStatusLabel: strings.TrimSpace(shop.SharedLoginStatusLabel),
			DataCompleteness:         "unknown",
			Source:                   "authorized_shop_projection",
		})
	}
	return profiles, nil
}

func normalizeShopProfiles(items []ShopProfileRecord) []ShopProfileRecord {
	profiles := make([]ShopProfileRecord, 0, len(items))
	for _, item := range items {
		item.ShopID = strings.TrimSpace(item.ShopID)
		item.ShopName = strings.TrimSpace(item.ShopName)
		item.ASMShopID = strings.TrimSpace(item.ASMShopID)
		item.ShopCode = strings.TrimSpace(item.ShopCode)
		item.ShopAlias = strings.TrimSpace(item.ShopAlias)
		item.FullShopName = strings.TrimSpace(item.FullShopName)
		item.PlatformCode = strings.TrimSpace(item.PlatformCode)
		item.PlatformName = strings.TrimSpace(item.PlatformName)
		item.PlatformSubtype = strings.TrimSpace(item.PlatformSubtype)
		item.ShopStatus = strings.TrimSpace(item.ShopStatus)
		item.ASMStatus = strings.TrimSpace(item.ASMStatus)
		item.AuthorizationStatus = strings.TrimSpace(item.AuthorizationStatus)
		item.AuthorizationStatusLabel = strings.TrimSpace(item.AuthorizationStatusLabel)
		item.OwnerName = strings.TrimSpace(item.OwnerName)
		item.OperatorName = strings.TrimSpace(item.OperatorName)
		item.OperatorUsername = strings.TrimSpace(item.OperatorUsername)
		item.BusinessManagerName = strings.TrimSpace(item.BusinessManagerName)
		item.BusinessManagerUsername = strings.TrimSpace(item.BusinessManagerUsername)
		item.Department = strings.TrimSpace(item.Department)
		item.SubCompanyName = strings.TrimSpace(item.SubCompanyName)
		item.ShopURL = strings.TrimSpace(item.ShopURL)
		item.ShopEmail = strings.TrimSpace(item.ShopEmail)
		item.ShopPhone = strings.TrimSpace(item.ShopPhone)
		item.LegalRepName = strings.TrimSpace(item.LegalRepName)
		item.BusinessLicense = strings.TrimSpace(item.BusinessLicense)
		item.UnifiedSocialCode = strings.TrimSpace(item.UnifiedSocialCode)
		item.RegisteredAddress = strings.TrimSpace(item.RegisteredAddress)
		item.CategoryIDs = trimStringSlice(item.CategoryIDs)
		item.CategoryNames = trimStringSlice(item.CategoryNames)
		item.BrandName = strings.TrimSpace(item.BrandName)
		item.BrandIDs = trimStringSlice(item.BrandIDs)
		item.AdvancedMemberName = strings.TrimSpace(item.AdvancedMemberName)
		item.TrustPassExpireAt = strings.TrimSpace(item.TrustPassExpireAt)
		item.JSTShopSummary = strings.TrimSpace(item.JSTShopSummary)
		item.MabangShopSummary = strings.TrimSpace(item.MabangShopSummary)
		item.ERPShopSummary = strings.TrimSpace(item.ERPShopSummary)
		item.AbnormalSummary = strings.TrimSpace(item.AbnormalSummary)
		item.TableSource = strings.TrimSpace(item.TableSource)
		item.MainCategory = strings.TrimSpace(item.MainCategory)
		item.DataCompleteness = strings.TrimSpace(item.DataCompleteness)
		item.SourceCreatedAt = strings.TrimSpace(item.SourceCreatedAt)
		item.SourceUpdatedAt = strings.TrimSpace(item.SourceUpdatedAt)
		item.LastSyncedAt = strings.TrimSpace(item.LastSyncedAt)
		item.Source = strings.TrimSpace(item.Source)
		if item.ASMStatus == "" {
			item.ASMStatus = "connected"
		}
		if item.DataCompleteness == "" {
			item.DataCompleteness = "unknown"
		}
		if item.Source == "" {
			item.Source = "asm_shop_profile"
		}
		profiles = append(profiles, item)
	}
	return profiles
}

func trimStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func (c *WorkspaceClient) FetchRuns(ctx context.Context, query RunQuery) (*RunsPayload, error) {
	params := make([]string, 0, 4)
	if query.Limit > 0 {
		params = append(params, "limit="+url.QueryEscape(fmt.Sprintf("%d", query.Limit)))
	}
	if strings.TrimSpace(query.Status) != "" {
		params = append(params, "status="+url.QueryEscape(strings.TrimSpace(query.Status)))
	}
	if strings.TrimSpace(query.ShopID) != "" {
		params = append(params, "shopId="+url.QueryEscape(strings.TrimSpace(query.ShopID)))
	}
	if strings.TrimSpace(query.FailureCode) != "" {
		params = append(params, "failureCode="+url.QueryEscape(strings.TrimSpace(query.FailureCode)))
	}
	path := "/local/runs"
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}
	var payload RunsPayload
	if err := c.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *WorkspaceClient) FetchRunEvents(ctx context.Context, runID string, limit int) (*RunEventsPayload, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	path := fmt.Sprintf("/local/runs/%s/events", urlPathEscape(runID))
	if limit > 0 {
		path += "?limit=" + url.QueryEscape(fmt.Sprintf("%d", limit))
	}
	var payload RunEventsPayload
	if err := c.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *WorkspaceClient) FetchOpenShopContext(ctx context.Context, shopID string) (*ShopOpenContext, error) {
	var payload ShopOpenContext
	path := fmt.Sprintf("/local/shops/%s/open-context", urlPathEscape(strings.TrimSpace(shopID)))
	if err := c.postJSON(ctx, path, nil, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *WorkspaceClient) ReportOpenShopResult(ctx context.Context, openRequestID string, request OpenReportRequest) error {
	path := fmt.Sprintf("/local/open-requests/%s/report", urlPathEscape(strings.TrimSpace(openRequestID)))
	return c.postJSON(ctx, path, request, nil)
}

func (s *WorkspaceService) FetchSummary(ctx context.Context) (*WorkspaceSummary, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchWorkspaceSummary(ctx)
}

func (s *WorkspaceService) FetchAuthorizedShops(ctx context.Context) ([]ShopInstanceProjection, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}

	shops, err := s.client.FetchAuthorizedShops(ctx)
	if err != nil {
		return nil, err
	}
	if s.reconciler != nil {
		if _, err := s.reconciler.ReconcileAuthorizedShops(shops); err != nil {
			return nil, err
		}
	}

	runtimeIndex := s.localRuntimeIndex()
	projected := make([]ShopInstanceProjection, 0, len(shops))
	for _, shop := range shops {
		profileID := buildProfileID(shop.PlatformCode, shop.ShopID)
		projected = append(projected, ProjectShopInstance(shop, runtimeIndex[profileID]))
	}

	return projected, nil
}

func (s *WorkspaceService) FetchShopProfiles(ctx context.Context) ([]ShopProfileRecord, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchShopProfiles(ctx)
}

func (s *WorkspaceService) FetchRuns(ctx context.Context, query RunQuery) (*RunsPayload, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchRuns(ctx, query)
}

func (s *WorkspaceService) FetchRunEvents(ctx context.Context, runID string, limit int) (*RunEventsPayload, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchRunEvents(ctx, runID, limit)
}

func (s *WorkspaceService) FetchRunEvidence(ctx context.Context, query RunQuery) (*RunEvidenceIndex, error) {
	runs, err := s.FetchRuns(ctx, query)
	if err != nil {
		return nil, err
	}
	index := BuildRunEvidenceIndex(runs.Items)
	return &index, nil
}

func (s *WorkspaceService) FetchOperationTasks(ctx context.Context, query OperationTaskQuery) (*OperationTasksPayload, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	shops, err := s.FetchAuthorizedShops(ctx)
	if err != nil {
		return nil, err
	}
	runs, err := s.FetchRuns(ctx, RunQuery{Limit: 200})
	if err != nil {
		return nil, err
	}
	evidence := BuildRunEvidenceIndex(runs.Items)
	payload := BuildOperationTasks(shops, evidence, query)
	return &payload, nil
}

func (s *WorkspaceService) FetchOpenShopContext(ctx context.Context, shopID string) (*ShopOpenContext, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchOpenShopContext(ctx, shopID)
}

func (s *WorkspaceService) ReportOpenShopResult(ctx context.Context, openRequestID string, request OpenReportRequest) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("workspace service is not configured")
	}
	return s.client.ReportOpenShopResult(ctx, openRequestID, request)
}

func (s *WorkspaceService) localRuntimeIndex() map[string]LocalRuntimeState {
	index := make(map[string]LocalRuntimeState)
	if s == nil || s.profileList == nil {
		return index
	}

	for _, profile := range s.profileList.List() {
		index[profile.ProfileId] = LocalRuntimeState{
			ProfileExists:  true,
			Running:        profile.Running,
			ReclaimPending: hasTag(profile.Tags, "managed:reclaim-pending"),
			CoreReady:      s.profileCoreReady(profile),
		}
	}

	return index
}

func (s *WorkspaceService) profileCoreReady(profile browser.Profile) bool {
	if s == nil || s.profileList == nil {
		return false
	}

	coreID := strings.TrimSpace(profile.CoreId)
	if coreID != "" {
		_, ok := s.profileList.GetCore(coreID)
		return ok
	}
	_, ok := s.profileList.GetDefaultCore()
	return ok
}

func hasTag(tags []string, target string) bool {
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

func (c *WorkspaceClient) getJSON(ctx context.Context, path string, dest interface{}) error {
	return c.requestJSON(ctx, http.MethodGet, path, nil, dest)
}

func (c *WorkspaceClient) postJSON(ctx context.Context, path string, body interface{}, dest interface{}) error {
	return c.requestJSON(ctx, http.MethodPost, path, body, dest)
}

func (c *WorkspaceClient) requestJSON(ctx context.Context, method string, path string, body interface{}, dest interface{}) error {
	if c == nil || c.baseURL == "" {
		return fmt.Errorf("workspace server base url is not configured")
	}

	var payloadReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode workspace request %s: %w", path, err)
		}
		payloadReader = strings.NewReader(string(raw))
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, payloadReader)
	if err != nil {
		return fmt.Errorf("create workspace request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request workspace endpoint %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("workspace endpoint %s returned status %d", path, resp.StatusCode)
	}

	var wrapped envelope[json.RawMessage]
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&wrapped); err != nil {
		return fmt.Errorf("decode workspace response %s: %w", path, err)
	}
	if dest == nil || len(wrapped.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(wrapped.Data, dest); err != nil {
		return fmt.Errorf("decode workspace payload %s: %w", path, err)
	}
	return nil
}

func isWorkspaceEndpointNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "returned status 404")
}

func urlPathEscape(value string) string {
	replacer := strings.NewReplacer("%", "%25", "/", "%2F", "?", "%3F", "#", "%23")
	return replacer.Replace(value)
}
