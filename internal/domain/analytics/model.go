package analytics

import "time"

// MessageTracking representa uma mensagem rastreada enviada pelo sistema
type MessageTracking struct {
	ID           string    `json:"id" db:"id"`
	InstanceID   string    `json:"instanceId" db:"instance_id"`
	MessageID    string    `json:"messageId" db:"message_id"`
	RemoteJID    string    `json:"remoteJid" db:"remote_jid"`
	CommunityJID string    `json:"communityJid,omitempty" db:"community_jid"`
	MessageType  string    `json:"messageType" db:"message_type"`
	Content      string    `json:"content,omitempty" db:"content"`
	MediaURL     string    `json:"mediaUrl,omitempty" db:"media_url"`
	Caption      string    `json:"caption,omitempty" db:"caption"`
	SentAt       time.Time `json:"sentAt" db:"sent_at"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

// MessageView representa uma visualização de mensagem
type MessageView struct {
	ID             string    `json:"id" db:"id"`
	MessageTrackID string    `json:"messageTrackId" db:"message_track_id"`
	ViewerJID      string    `json:"viewerJid" db:"viewer_jid"`
	ViewerName     string    `json:"viewerName,omitempty" db:"viewer_name"`
	ViewedAt       time.Time `json:"viewedAt" db:"viewed_at"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
}

// MessageReaction representa uma reação a uma mensagem
type MessageReaction struct {
	ID             string    `json:"id" db:"id"`
	MessageTrackID string    `json:"messageTrackId" db:"message_track_id"`
	ReactorJID     string    `json:"reactorJid" db:"reactor_jid"`
	ReactorName    string    `json:"reactorName,omitempty" db:"reactor_name"`
	Reaction       string    `json:"reaction" db:"reaction"`
	ReactedAt      time.Time `json:"reactedAt" db:"reacted_at"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
}

// MessageMetrics representa as métricas agregadas de uma mensagem
type MessageMetrics struct {
	MessageTracking MessageTracking   `json:"message"`
	ViewCount       int               `json:"viewCount"`
	ReactionCount   int               `json:"reactionCount"`
	Views           []MessageView     `json:"views,omitempty"`
	Reactions       []MessageReaction `json:"reactions,omitempty"`
}

// MessageMetricsSummary representa um resumo das métricas
type MessageMetricsSummary struct {
	MessageID     string         `json:"messageId"`
	RemoteJID     string         `json:"remoteJid"`
	MessageType   string         `json:"messageType"`
	SentAt        time.Time      `json:"sentAt"`
	ViewCount     int            `json:"viewCount"`
	ReactionCount int            `json:"reactionCount"`
	TopReactions  map[string]int `json:"topReactions"`
}

// CreateMessageTrackingInput entrada para criar rastreamento
type CreateMessageTrackingInput struct {
	InstanceID   string
	MessageID    string
	RemoteJID    string
	CommunityJID string
	MessageType  string
	Content      string
	MediaURL     string
	Caption      string
	SentAt       time.Time
}

// CreateMessageViewInput entrada para registrar visualização
type CreateMessageViewInput struct {
	MessageTrackID string
	ViewerJID      string
	ViewerName     string
	ViewedAt       time.Time
}

// CreateMessageReactionInput entrada para registrar reação
type CreateMessageReactionInput struct {
	MessageTrackID string
	ReactorJID     string
	ReactorName    string
	Reaction       string
	ReactedAt      time.Time
}
