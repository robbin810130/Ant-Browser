package workspace

import (
	"ant-chrome/backend/internal/browser"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type WorkspaceClient struct {
	baseURL    string
	httpClient *http.Client
}

type WorkspaceService struct {
	client      *WorkspaceClient
	profileList profileLister
}

type profileLister interface {
	List() []browser.Profile
}

type envelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type shopsPayload struct {
	Items []ShopRecord `json:"items"`
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

func NewService(client *WorkspaceClient, profileList profileLister) *WorkspaceService {
	return &WorkspaceService{
		client:      client,
		profileList: profileList,
	}
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

	runtimeIndex := s.localRuntimeIndex()
	projected := make([]ShopInstanceProjection, 0, len(shops))
	for _, shop := range shops {
		profileID := buildProfileID(shop.PlatformCode, shop.ShopID)
		projected = append(projected, ProjectShopInstance(shop, runtimeIndex[profileID]))
	}

	return projected, nil
}

func (s *WorkspaceService) localRuntimeIndex() map[string]LocalRuntimeState {
	index := make(map[string]LocalRuntimeState)
	if s == nil || s.profileList == nil {
		return index
	}

	for _, profile := range s.profileList.List() {
		index[profile.ProfileId] = LocalRuntimeState{
			ProfileExists: true,
			InstanceID:    profile.ProfileId,
			Running:       profile.Running,
		}
	}

	return index
}

func (c *WorkspaceClient) getJSON(ctx context.Context, path string, dest interface{}) error {
	if c == nil || c.baseURL == "" {
		return fmt.Errorf("workspace server base url is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create workspace request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

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
	if len(wrapped.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(wrapped.Data, dest); err != nil {
		return fmt.Errorf("decode workspace payload %s: %w", path, err)
	}
	return nil
}
