package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type WorkspaceSummary struct {
	Status              string `json:"status"`
	AgentStatus         string `json:"agentStatus"`
	SessionReady        bool   `json:"sessionReady"`
	ServerReachable     bool   `json:"serverReachable"`
	AntRuntimeReachable bool   `json:"antRuntimeReachable"`
	ActiveRunCount      int    `json:"activeRunCount"`
	DeviceID            string `json:"deviceId"`
	DeviceStatus        string `json:"deviceStatus"`
}

type ShopRecord struct {
	ShopID                 string `json:"shopId"`
	ShopName               string `json:"shopName"`
	PlatformCode           string `json:"platformCode"`
	SharedLoginStatus      string `json:"sharedLoginStatus"`
	SharedLoginStatusLabel string `json:"sharedLoginStatusLabel"`
	LastValidatedAt        string `json:"lastValidatedAt,omitempty"`
	LastOpenedAt           string `json:"lastOpenedAt,omitempty"`
	LastOpenFailureCode    string `json:"lastOpenFailureCode,omitempty"`
	LastOpenFailureMessage string `json:"lastOpenFailureMessage,omitempty"`
	LastOpenFailedAt       string `json:"lastOpenFailedAt,omitempty"`
}

type LocalRuntimeState struct {
	ProfileExists  bool
	InstanceID     string
	Running        bool
	ReclaimPending bool
	CoreReady      bool
}

type ShopInstanceProjection struct {
	ShopID                 string `json:"shopId"`
	ShopName               string `json:"shopName"`
	PlatformCode           string `json:"platformCode"`
	ProfileID              string `json:"profileId"`
	InstanceID             string `json:"instanceId"`
	SharedLoginStatus      string `json:"sharedLoginStatus"`
	SharedLoginStatusLabel string `json:"sharedLoginStatusLabel"`
	InstanceRunning        bool   `json:"instanceRunning"`
	ProfileExists          bool   `json:"profileExists"`
	ReclaimPending         bool   `json:"reclaimPending"`
	CoreReady              bool   `json:"coreReady"`
	LastValidatedAt        string `json:"lastValidatedAt,omitempty"`
	LastOpenedAt           string `json:"lastOpenedAt,omitempty"`
	LastOpenFailureCode    string `json:"lastOpenFailureCode,omitempty"`
	LastOpenFailureMessage string `json:"lastOpenFailureMessage,omitempty"`
	LastOpenFailedAt       string `json:"lastOpenFailedAt,omitempty"`
}

type ReconcileSummary struct {
	CreatedProfileIDs        []string `json:"createdProfileIds,omitempty"`
	UpdatedProfileIDs        []string `json:"updatedProfileIds,omitempty"`
	ReclaimedProfileIDs      []string `json:"reclaimedProfileIds,omitempty"`
	PendingReclaimProfileIDs []string `json:"pendingReclaimProfileIds,omitempty"`
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
	Origin         string            `json:"origin"`
	Scope          string            `json:"scope"`
	LocalStorage   map[string]string `json:"localStorage"`
	SessionStorage map[string]string `json:"sessionStorage"`
}

type SessionBundle struct {
	PlatformCode     string                `json:"platformCode"`
	CapturedAt       string                `json:"capturedAt"`
	CaptureStartedAt string                `json:"captureStartedAt"`
	LastObservedURL  string                `json:"lastObservedUrl"`
	UserAgent        string                `json:"userAgent"`
	Cookies          []SessionCookie       `json:"cookies"`
	Storages         []SessionStorageEntry `json:"storages"`
}

type ShopLaunchContext struct {
	TargetURL          string        `json:"targetUrl"`
	SessionBundle      SessionBundle `json:"sessionBundle"`
	SuccessURLPatterns []string      `json:"successUrlPatterns"`
	LoginURLPatterns   []string      `json:"loginUrlPatterns"`
}

type ShopRuntimeConfig struct {
	ManagedMode bool `json:"managedMode"`
}

type ShopProfile struct {
	ProfileID  string `json:"profileId"`
	ProfileKey string `json:"profileKey"`
}

type ShopDescriptor struct {
	ShopID            string `json:"shopId"`
	ShopName          string `json:"shopName"`
	PlatformCode      string `json:"platformCode"`
	SharedLoginStatus string `json:"sharedLoginStatus"`
	LastOpenedAt      string `json:"lastOpenedAt"`
}

type ShopOpenContext struct {
	OpenRequestID string            `json:"openRequestId"`
	Shop          ShopDescriptor    `json:"shop"`
	Profile       ShopProfile       `json:"profile"`
	LaunchContext ShopLaunchContext `json:"launchContext"`
	RuntimeConfig ShopRuntimeConfig `json:"runtimeConfig"`
}

type OpenReportRuntime struct {
	PID        int    `json:"pid"`
	DebugPort  int    `json:"debugPort"`
	CurrentURL string `json:"currentUrl,omitempty"`
	PageTitle  string `json:"pageTitle,omitempty"`
}

type OpenReportRequest struct {
	Status         string             `json:"status"`
	Runtime        *OpenReportRuntime `json:"runtime,omitempty"`
	FailureCode    string             `json:"failureCode,omitempty"`
	FailureMessage string             `json:"failureMessage,omitempty"`
}

type ShopProfileRecord struct {
	ShopID                   string   `json:"shopId"`
	ShopName                 string   `json:"shopName"`
	ASMShopID                string   `json:"asmShopId"`
	ShopCode                 string   `json:"shopCode"`
	ShopAlias                string   `json:"shopAlias"`
	FullShopName             string   `json:"fullShopName"`
	PlatformCode             string   `json:"platformCode"`
	PlatformName             string   `json:"platformName"`
	PlatformSubtype          string   `json:"platformSubtype"`
	ShopStatusCode           int      `json:"shopStatusCode"`
	ShopStatus               string   `json:"shopStatus"`
	ASMStatus                string   `json:"asmStatus"`
	AuthorizationStatus      string   `json:"authorizationStatus"`
	AuthorizationStatusLabel string   `json:"authorizationStatusLabel"`
	OwnerName                string   `json:"ownerName"`
	OperatorName             string   `json:"operatorName"`
	OperatorUsername         string   `json:"operatorUsername"`
	BusinessManagerName      string   `json:"businessManagerName"`
	BusinessManagerUsername  string   `json:"businessManagerUsername"`
	Department               string   `json:"department"`
	SubCompanyName           string   `json:"subCompanyName"`
	ShopURL                  string   `json:"shopUrl"`
	ShopEmail                string   `json:"shopEmail"`
	ShopPhone                string   `json:"shopPhone"`
	LegalRepName             string   `json:"legalRepName"`
	BusinessLicense          string   `json:"businessLicense"`
	UnifiedSocialCode        string   `json:"unifiedSocialCode"`
	RegisteredAddress        string   `json:"registeredAddress"`
	CategoryIDs              []string `json:"categoryIds"`
	CategoryNames            []string `json:"categoryNames"`
	BrandName                string   `json:"brandName"`
	BrandIDs                 []string `json:"brandIds"`
	AdvancedMember           int      `json:"advancedMember"`
	AdvancedMemberName       string   `json:"advancedMemberName"`
	TrustPassExpireAt        string   `json:"trustPassExpireAt"`
	JSTShopCount             int      `json:"jstShopCount"`
	JSTShopSummary           string   `json:"jstShopSummary"`
	MabangShopCount          int      `json:"mabangShopCount"`
	MabangShopSummary        string   `json:"mabangShopSummary"`
	ERPShopCount             int      `json:"erpShopCount"`
	ERPShopSummary           string   `json:"erpShopSummary"`
	AbnormalCount            int      `json:"abnormalCount"`
	AbnormalSummary          string   `json:"abnormalSummary"`
	TableSource              string   `json:"tableSource"`
	IsPush                   int      `json:"isPush"`
	MainCategory             string   `json:"mainCategory"`
	DataCompleteness         string   `json:"dataCompleteness"`
	SourceCreatedAt          string   `json:"sourceCreatedAt"`
	SourceUpdatedAt          string   `json:"sourceUpdatedAt"`
	LastSyncedAt             string   `json:"lastSyncedAt"`
	Source                   string   `json:"source"`
}

func (record *ShopProfileRecord) UnmarshalJSON(data []byte) error {
	type alias ShopProfileRecord
	aux := struct {
		CategoryIDs   json.RawMessage `json:"categoryIds"`
		CategoryNames json.RawMessage `json:"categoryNames"`
		BrandIDs      json.RawMessage `json:"brandIds"`
		*alias
	}{
		alias: (*alias)(record),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var err error
	if len(aux.CategoryIDs) > 0 {
		record.CategoryIDs, err = decodeFlexibleStringList(aux.CategoryIDs)
		if err != nil {
			return fmt.Errorf("categoryIds: %w", err)
		}
	}
	if len(aux.CategoryNames) > 0 {
		record.CategoryNames, err = decodeFlexibleStringList(aux.CategoryNames)
		if err != nil {
			return fmt.Errorf("categoryNames: %w", err)
		}
	}
	if len(aux.BrandIDs) > 0 {
		record.BrandIDs, err = decodeFlexibleStringList(aux.BrandIDs)
		if err != nil {
			return fmt.Errorf("brandIds: %w", err)
		}
	}
	return nil
}

func decodeFlexibleStringList(data []byte) ([]string, error) {
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var values []any
	if err := decoder.Decode(&values); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		var text string
		switch item := value.(type) {
		case nil:
			continue
		case string:
			text = item
		case json.Number:
			text = item.String()
		default:
			return nil, fmt.Errorf("unsupported item type %T", value)
		}
		text = strings.TrimSpace(text)
		if text != "" {
			result = append(result, text)
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

type RunQuery struct {
	Limit       int
	Status      string
	ShopID      string
	FailureCode string
}

type RunRuntime struct {
	PID        int    `json:"pid"`
	DebugPort  int    `json:"debugPort"`
	CurrentURL string `json:"currentUrl"`
	PageTitle  string `json:"pageTitle"`
	TargetURL  string `json:"targetUrl"`
}

type RunRecord struct {
	RunID                string      `json:"runId"`
	TaskID               string      `json:"taskId"`
	ShopID               string      `json:"shopId"`
	TaskType             string      `json:"taskType"`
	Status               string      `json:"status"`
	StatusLabel          string      `json:"statusLabel"`
	StartedAt            string      `json:"startedAt"`
	FinishedAt           string      `json:"finishedAt"`
	ProfileID            string      `json:"profileId"`
	Runtime              *RunRuntime `json:"runtime,omitempty"`
	BindSessionID        string      `json:"bindSessionId"`
	ManualActionRequired bool        `json:"manualActionRequired"`
	ChallengeType        string      `json:"challengeType"`
	FailureCode          string      `json:"failureCode"`
	FailureMessage       string      `json:"failureMessage"`
}

type RunsPayload struct {
	Items []RunRecord `json:"items"`
	Total int         `json:"total"`
}

type RunEvent struct {
	EventID   string         `json:"eventId"`
	Stage     string         `json:"stage"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt string         `json:"createdAt"`
}

type RunEventsPayload struct {
	RunID string     `json:"runId"`
	Items []RunEvent `json:"items"`
	Total int        `json:"total"`
}

type OperationTaskQuery struct {
	Limit    int
	Status   string
	ShopID   string
	TaskType string
}

type OperationTaskRecord struct {
	TaskID         string `json:"taskId"`
	ShopID         string `json:"shopId"`
	ShopName       string `json:"shopName"`
	TaskType       string `json:"taskType"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	BlockedReason  string `json:"blockedReason"`
	FailureMessage string `json:"failureMessage"`
	UpdatedAt      string `json:"updatedAt"`
	RunID          string `json:"runId"`
	FailureCode    string `json:"failureCode"`
}

type OperationTasksPayload struct {
	Items []OperationTaskRecord `json:"items"`
	Total int                   `json:"total"`
}
