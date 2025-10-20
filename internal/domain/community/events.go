package community

import "time"

// MembershipEvent representa uma mudança em tempo real na comunidade.
type MembershipEvent struct {
	Timestamp    time.Time         `json:"timestamp"`
	CommunityID  string            `json:"communityId"`
	Action       string            `json:"action"`
	Payload      MembershipPayload `json:"payload"`
	TotalMembers int               `json:"totalMembers"`
}

// MembershipPayload traz os dados do usuário afetado.
type MembershipPayload struct {
	UserID    string     `json:"userId"`
	UserName  string     `json:"userName,omitempty"`
	UserPhone string     `json:"userPhone,omitempty"`
	JoinedAt  *time.Time `json:"joinedAt,omitempty"`
	LeftAt    *time.Time `json:"leftAt,omitempty"`
}
