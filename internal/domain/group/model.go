package group

import "time"

// CreateInput mirrors Evolution API payload for creating a WhatsApp group.
type CreateInput struct {
	InstanceID          string   `json:"instanceId"`
	Subject             string   `json:"subject"`
	Description         string   `json:"description,omitempty"`
	Participants        []string `json:"participants"`
	PromoteParticipants bool     `json:"promoteParticipants,omitempty"`
	ProfilePicture      string   `json:"profilePicture,omitempty"`
}

// UpdatePictureInput carries the payload for updating a group's profile photo.
type UpdatePictureInput struct {
	InstanceID string `json:"instanceId"`
	GroupJID   string `json:"-"`
	Image      string `json:"image"`
}

// UpdateDescriptionInput mirrors Evolution API payload for updating group description.
type UpdateDescriptionInput struct {
	InstanceID  string `json:"instanceId"`
	GroupJID    string `json:"-"`
	Description string `json:"description"`
}

// InviteInput captures the common parameters for invite-related operations.
type InviteInput struct {
	InstanceID string `json:"instanceId"`
	GroupJID   string `json:"groupJid"`
}

// InviteResponse matches Evolution API invite payload.
type InviteResponse struct {
	InviteURL  string `json:"inviteUrl"`
	InviteCode string `json:"inviteCode"`
}

// SendInviteInput represents the request body for sending invites to numbers.
type SendInviteInput struct {
	InstanceID  string   `json:"instanceId"`
	GroupJID    string   `json:"groupJid"`
	Description string   `json:"description,omitempty"`
	Numbers     []string `json:"numbers"`
}

// SendInviteOutput mirrors Evolution API response when sending invites.
type SendInviteOutput struct {
	Send      bool   `json:"send"`
	InviteURL string `json:"inviteUrl"`
}

// FetchAllGroupsInput captures query parameters for fetching all groups.
type FetchAllGroupsInput struct {
	InstanceID      string `json:"instanceId"`
	GetParticipants bool   `json:"getParticipants"`
}

// FindGroupByJIDInput represents the input for finding a group by its JID.
type FindGroupByJIDInput struct {
	InstanceID string `json:"instanceId"`
	GroupJID   string `json:"groupJid"`
}

// FindGroupByInviteCodeInput represents the input for finding a group by invite code.
type FindGroupByInviteCodeInput struct {
	InstanceID string `json:"instanceId"`
	InviteCode string `json:"inviteCode"`
}

// FindParticipantsInput represents the input for fetching group participants.
type FindParticipantsInput struct {
	InstanceID string `json:"instanceId"`
	GroupJID   string `json:"groupJid"`
}

// FindParticipantsOutput represents the response with participant list.
type FindParticipantsOutput struct {
	Participants []GroupParticipant `json:"participants"`
}

// UpdateParticipantInput represents the input for updating group participants.
type UpdateParticipantInput struct {
	InstanceID   string   `json:"instanceId"`
	GroupJID     string   `json:"groupJid"`
	Action       string   `json:"action"` // add, remove, promote, demote
	Participants []string `json:"participants"`
}

// UpdateSettingInput represents the input for updating group settings.
type UpdateSettingInput struct {
	InstanceID string `json:"instanceId"`
	GroupJID   string `json:"groupJid"`
	Action     string `json:"action"` // announcement, not_announcement, locked, unlocked
}

// ToggleEphemeralInput represents the input for toggling ephemeral messages.
type ToggleEphemeralInput struct {
	InstanceID string `json:"instanceId"`
	GroupJID   string `json:"groupJid"`
	Expiration int    `json:"expiration"` // Time in seconds (0 to disable)
}

// LeaveGroupInput represents the input for leaving a group.
type LeaveGroupInput struct {
	InstanceID string `json:"instanceId"`
	GroupJID   string `json:"groupJid"`
}

// Group represents the response structure returned after creating/fetching a group.
type Group struct {
	ID                  string             `json:"id"`
	Subject             string             `json:"subject"`
	SubjectOwner        string             `json:"subjectOwner,omitempty"`
	SubjectTime         time.Time          `json:"subjectTime,omitempty"`
	PictureURL          string             `json:"pictureUrl,omitempty"`
	Size                int                `json:"size"`
	Creation            time.Time          `json:"creation,omitempty"`
	Owner               string             `json:"owner,omitempty"`
	Description         string             `json:"desc,omitempty"`
	DescriptionID       string             `json:"descId,omitempty"`
	Restrict            bool               `json:"restrict"`
	Announce            bool               `json:"announce"`
	Participants        []GroupParticipant `json:"participants,omitempty"`
	IsCommunity         bool               `json:"isCommunity"`
	IsCommunityAnnounce bool               `json:"isCommunityAnnounce"`
	LinkedParent        string             `json:"linkedParent,omitempty"`
}

// GroupParticipant carries participant metadata similar to Evolution API output.
type GroupParticipant struct {
	JID          string `json:"id"`
	Phone        string `json:"phone,omitempty"`
	IsAdmin      bool   `json:"isAdmin"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
	DisplayName  string `json:"displayName,omitempty"`
	Error        int    `json:"error,omitempty"`
}
