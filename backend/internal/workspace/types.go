package workspace

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
	ShopID              string `json:"shopId"`
	ShopName            string `json:"shopName"`
	PlatformCode        string `json:"platformCode"`
	ASMStatus           string `json:"asmStatus"`
	AuthorizationStatus string `json:"authorizationStatus"`
	OwnerName           string `json:"ownerName"`
	MainCategory        string `json:"mainCategory"`
	DataCompleteness    string `json:"dataCompleteness"`
	LastSyncedAt        string `json:"lastSyncedAt"`
	Source              string `json:"source"`
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
