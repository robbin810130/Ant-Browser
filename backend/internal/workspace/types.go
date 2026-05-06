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
	ProfileExists bool
	InstanceID    string
	Running       bool
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
}
