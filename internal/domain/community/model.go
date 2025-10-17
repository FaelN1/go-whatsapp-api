package community

import "time"

// CreateInput contains the payload required to create a community.
type CreateInput struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Image        string   `json:"image,omitempty"`
	Participants []string `json:"participants,omitempty"`
}

// Community holds basic community metadata returned to clients.
type Community struct {
	JID                string    `json:"jid"`
	Name               string    `json:"name"`
	Description        string    `json:"description,omitempty"`
	AnnouncementJID    string    `json:"announcement_jid,omitempty"`
	DefaultSubGroupJID string    `json:"default_subgroup_jid,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	IsLocked           bool      `json:"is_locked"`
	IsAnnouncementOnly bool      `json:"is_announcement_only"`
	MemberCount        int       `json:"member_count"`
}

// Member represents a user that belongs to a community.
type Member struct {
	JID         string `json:"jid"`
	Phone       string `json:"phone,omitempty"`
	IsAdmin     bool   `json:"is_admin"`
	DisplayName string `json:"display_name,omitempty"`
}

// SendAnnouncementInput carries the request body to broadcast an announcement to the community announcement group.
type SendAnnouncementInput struct {
	Text string `json:"text"`
}
