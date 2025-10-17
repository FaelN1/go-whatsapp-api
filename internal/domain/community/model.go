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
	JID         string    `json:"jid"`
	Phone       string    `json:"phone,omitempty"`
	IsAdmin     bool      `json:"is_admin"`
	DisplayName string    `json:"display_name,omitempty"`
	FirstSeen   time.Time `json:"first_seen,omitempty"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
}

// FormerMember represents a user that previously belonged to a community.
type FormerMember struct {
	Member
	LeftAt time.Time `json:"left_at"`
}

// Members aggregates current and former members for a community.
type Members struct {
	Current []Member       `json:"current"`
	Former  []FormerMember `json:"former,omitempty"`
}

// SendAnnouncementInput carries the request body to broadcast an announcement to one or more community announcement groups.
type SendAnnouncementInput struct {
	Text        string   `json:"text,omitempty"`
	Caption     string   `json:"caption,omitempty"`
	Media       string   `json:"media,omitempty"`
	MediaType   string   `json:"mediaType,omitempty"`
	MimeType    string   `json:"mimeType,omitempty"`
	FileName    string   `json:"fileName,omitempty"`
	Communities []string `json:"communities,omitempty"`
}
