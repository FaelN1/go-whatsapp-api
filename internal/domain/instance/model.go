package instance

import "time"

type ID string

type InstanceSettings struct {
	RejectCall      bool   `json:"rejectCall"`
	MsgCall         string `json:"msgCall"`
	GroupsIgnore    bool   `json:"groupsIgnore"`
	AlwaysOnline    bool   `json:"alwaysOnline"`
	ReadMessages    bool   `json:"readMessages"`
	ReadStatus      bool   `json:"readStatus"`
	SyncFullHistory bool   `json:"syncFullHistory"`
}

type InstanceWebhook struct {
	URL      string            `json:"url"`
	ByEvents bool              `json:"byEvents"`
	Base64   bool              `json:"base64"`
	Headers  map[string]string `json:"headers,omitempty"`
	Events   []string          `json:"events,omitempty"`
	Enabled  bool              `json:"enabled"`
}

type Instance struct {
	ID          ID               `json:"id"`
	Name        string           `json:"instanceName"`
	Token       string           `json:"token"`
	Number      string           `json:"number,omitempty"`
	Integration string           `json:"integration,omitempty"`
	WebhookURL  string           `json:"webhookUrl,omitempty"`
	Settings    InstanceSettings `json:"settings"`
	Webhook     InstanceWebhook  `json:"webhook"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	Status      string           `json:"status"` // open, closed, disconnected
}

// InstanceListResponse represents the full instance response following Evolution API format
type InstanceListResponse struct {
	ID                      string                  `json:"id"`
	Name                    string                  `json:"name"`
	ConnectionStatus        string                  `json:"connectionStatus"`
	OwnerJID                string                  `json:"ownerJid,omitempty"`
	ProfileName             string                  `json:"profileName,omitempty"`
	ProfilePicURL           string                  `json:"profilePicUrl,omitempty"`
	Integration             string                  `json:"integration"`
	Number                  string                  `json:"number"`
	Token                   string                  `json:"token"`
	ClientName              string                  `json:"clientName"`
	DisconnectionReasonCode *int                    `json:"disconnectionReasonCode"`
	DisconnectionObject     *string                 `json:"disconnectionObject"`
	DisconnectionAt         *time.Time              `json:"disconnectionAt"`
	CreatedAt               time.Time               `json:"createdAt"`
	UpdatedAt               time.Time               `json:"updatedAt"`
	Setting                 *InstanceSettingDetails `json:"Setting,omitempty"`
	Count                   *InstanceCount          `json:"_count,omitempty"`
}

// InstanceSettingDetails represents the Setting object with additional metadata
type InstanceSettingDetails struct {
	ID              string    `json:"id"`
	RejectCall      bool      `json:"rejectCall"`
	MsgCall         string    `json:"msgCall"`
	GroupsIgnore    bool      `json:"groupsIgnore"`
	AlwaysOnline    bool      `json:"alwaysOnline"`
	ReadMessages    bool      `json:"readMessages"`
	ReadStatus      bool      `json:"readStatus"`
	SyncFullHistory bool      `json:"syncFullHistory"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	InstanceID      string    `json:"instanceId"`
}

// InstanceCount represents message/contact/chat counts
type InstanceCount struct {
	Message int `json:"Message"`
	Contact int `json:"Contact"`
	Chat    int `json:"Chat"`
}

type CreateInstanceInput struct {
	InstanceName    string            `json:"instanceName"`
	Token           string            `json:"token"`
	Number          string            `json:"number"`
	QRCode          bool              `json:"qrcode"`
	Integration     string            `json:"integration"`
	Settings        *InstanceSettings `json:"settings"`
	Webhook         *InstanceWebhook  `json:"webhook"`
	WebhookURL      string            `json:"webhookUrl"`
	RejectCall      *bool             `json:"rejectCall"`
	MsgCall         *string           `json:"msgCall"`
	GroupsIgnore    *bool             `json:"groupsIgnore"`
	AlwaysOnline    *bool             `json:"alwaysOnline"`
	ReadMessages    *bool             `json:"readMessages"`
	ReadStatus      *bool             `json:"readStatus"`
	SyncFullHistory *bool             `json:"syncFullHistory"`
}

type SetWebhookInput struct {
	Enabled         bool              `json:"enabled"`
	URL             string            `json:"url"`
	WebhookByEvents bool              `json:"webhookByEvents"`
	WebhookBase64   bool              `json:"webhookBase64"`
	Headers         map[string]string `json:"headers,omitempty"`
	Events          []string          `json:"events"`
}

type SetSettingsInput struct {
	RejectCall      bool   `json:"rejectCall"`
	MsgCall         string `json:"msgCall"`
	GroupsIgnore    bool   `json:"groupsIgnore"`
	AlwaysOnline    bool   `json:"alwaysOnline"`
	ReadMessages    bool   `json:"readMessages"`
	ReadStatus      bool   `json:"readStatus"`
	SyncFullHistory bool   `json:"syncFullHistory"`
}
