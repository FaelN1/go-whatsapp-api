package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/analytics"
	"github.com/google/uuid"
)

type AnalyticsRepository interface {
	CreateMessageTracking(ctx context.Context, in analytics.CreateMessageTrackingInput) (*analytics.MessageTracking, error)
	GetMessageTracking(ctx context.Context, messageID string) (*analytics.MessageTracking, error)
	GetMessageTrackingByID(ctx context.Context, id string) (*analytics.MessageTracking, error)

	CreateMessageView(ctx context.Context, in analytics.CreateMessageViewInput) (*analytics.MessageView, error)
	GetMessageViews(ctx context.Context, messageTrackID string) ([]analytics.MessageView, error)

	CreateMessageReaction(ctx context.Context, in analytics.CreateMessageReactionInput) (*analytics.MessageReaction, error)
	UpdateMessageReaction(ctx context.Context, messageTrackID, reactorJID, reaction string, reactedAt time.Time) error
	DeleteMessageReaction(ctx context.Context, messageTrackID, reactorJID string) error
	GetMessageReactions(ctx context.Context, messageTrackID string) ([]analytics.MessageReaction, error)

	GetMessageMetrics(ctx context.Context, messageTrackID string) (*analytics.MessageMetrics, error)
	GetInstanceMessageMetrics(ctx context.Context, instanceID string, limit, offset int) ([]analytics.MessageMetricsSummary, error)
}

type analyticsRepository struct {
	db *sql.DB
}

func NewAnalyticsRepository(db *sql.DB) AnalyticsRepository {
	return &analyticsRepository{db: db}
}

func (r *analyticsRepository) CreateMessageTracking(ctx context.Context, in analytics.CreateMessageTrackingInput) (*analytics.MessageTracking, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO message_tracking (
			id, instance_id, message_id, remote_jid, community_jid, 
			message_type, content, media_url, caption, sent_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, instance_id, message_id, remote_jid, community_jid, 
		          message_type, content, media_url, caption, sent_at, created_at
	`

	tracking := &analytics.MessageTracking{}
	err := r.db.QueryRowContext(
		ctx, query,
		id, in.InstanceID, in.MessageID, in.RemoteJID, in.CommunityJID,
		in.MessageType, in.Content, in.MediaURL, in.Caption, in.SentAt, now,
	).Scan(
		&tracking.ID, &tracking.InstanceID, &tracking.MessageID, &tracking.RemoteJID, &tracking.CommunityJID,
		&tracking.MessageType, &tracking.Content, &tracking.MediaURL, &tracking.Caption, &tracking.SentAt, &tracking.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create message tracking: %w", err)
	}

	return tracking, nil
}

func (r *analyticsRepository) GetMessageTracking(ctx context.Context, messageID string) (*analytics.MessageTracking, error) {
	query := `
		SELECT id, instance_id, message_id, remote_jid, community_jid, 
		       message_type, content, media_url, caption, sent_at, created_at
		FROM message_tracking
		WHERE message_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	tracking := &analytics.MessageTracking{}
	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&tracking.ID, &tracking.InstanceID, &tracking.MessageID, &tracking.RemoteJID, &tracking.CommunityJID,
		&tracking.MessageType, &tracking.Content, &tracking.MediaURL, &tracking.Caption, &tracking.SentAt, &tracking.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message tracking: %w", err)
	}

	return tracking, nil
}

func (r *analyticsRepository) GetMessageTrackingByID(ctx context.Context, id string) (*analytics.MessageTracking, error) {
	query := `
		SELECT id, instance_id, message_id, remote_jid, community_jid, 
		       message_type, content, media_url, caption, sent_at, created_at
		FROM message_tracking
		WHERE id = $1
	`

	tracking := &analytics.MessageTracking{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tracking.ID, &tracking.InstanceID, &tracking.MessageID, &tracking.RemoteJID, &tracking.CommunityJID,
		&tracking.MessageType, &tracking.Content, &tracking.MediaURL, &tracking.Caption, &tracking.SentAt, &tracking.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message tracking by id: %w", err)
	}

	return tracking, nil
}

func (r *analyticsRepository) CreateMessageView(ctx context.Context, in analytics.CreateMessageViewInput) (*analytics.MessageView, error) {
	id := uuid.New().String()
	now := time.Now()

	// Verificar se já existe visualização deste usuário
	checkQuery := `SELECT id FROM message_views WHERE message_track_id = $1 AND viewer_jid = $2`
	var existingID string
	err := r.db.QueryRowContext(ctx, checkQuery, in.MessageTrackID, in.ViewerJID).Scan(&existingID)
	if err == nil {
		// Já existe, apenas atualizar o timestamp
		updateQuery := `
			UPDATE message_views 
			SET viewed_at = $1, viewer_name = $2
			WHERE id = $3
			RETURNING id, message_track_id, viewer_jid, viewer_name, viewed_at, created_at
		`
		view := &analytics.MessageView{}
		err = r.db.QueryRowContext(ctx, updateQuery, in.ViewedAt, in.ViewerName, existingID).Scan(
			&view.ID, &view.MessageTrackID, &view.ViewerJID, &view.ViewerName, &view.ViewedAt, &view.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update message view: %w", err)
		}
		return view, nil
	}

	query := `
		INSERT INTO message_views (
			id, message_track_id, viewer_jid, viewer_name, viewed_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, message_track_id, viewer_jid, viewer_name, viewed_at, created_at
	`

	view := &analytics.MessageView{}
	err = r.db.QueryRowContext(
		ctx, query,
		id, in.MessageTrackID, in.ViewerJID, in.ViewerName, in.ViewedAt, now,
	).Scan(
		&view.ID, &view.MessageTrackID, &view.ViewerJID, &view.ViewerName, &view.ViewedAt, &view.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create message view: %w", err)
	}

	return view, nil
}

func (r *analyticsRepository) GetMessageViews(ctx context.Context, messageTrackID string) ([]analytics.MessageView, error) {
	query := `
		SELECT id, message_track_id, viewer_jid, viewer_name, viewed_at, created_at
		FROM message_views
		WHERE message_track_id = $1
		ORDER BY viewed_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, messageTrackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message views: %w", err)
	}
	defer rows.Close()

	views := []analytics.MessageView{}
	for rows.Next() {
		var view analytics.MessageView
		err := rows.Scan(
			&view.ID, &view.MessageTrackID, &view.ViewerJID, &view.ViewerName, &view.ViewedAt, &view.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message view: %w", err)
		}
		views = append(views, view)
	}

	return views, nil
}

func (r *analyticsRepository) CreateMessageReaction(ctx context.Context, in analytics.CreateMessageReactionInput) (*analytics.MessageReaction, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO message_reactions (
			id, message_track_id, reactor_jid, reactor_name, reaction, reacted_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, message_track_id, reactor_jid, reactor_name, reaction, reacted_at, created_at
	`

	reaction := &analytics.MessageReaction{}
	err := r.db.QueryRowContext(
		ctx, query,
		id, in.MessageTrackID, in.ReactorJID, in.ReactorName, in.Reaction, in.ReactedAt, now,
	).Scan(
		&reaction.ID, &reaction.MessageTrackID, &reaction.ReactorJID, &reaction.ReactorName,
		&reaction.Reaction, &reaction.ReactedAt, &reaction.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create message reaction: %w", err)
	}

	return reaction, nil
}

func (r *analyticsRepository) UpdateMessageReaction(ctx context.Context, messageTrackID, reactorJID, reaction string, reactedAt time.Time) error {
	query := `
		UPDATE message_reactions 
		SET reaction = $1, reacted_at = $2
		WHERE message_track_id = $3 AND reactor_jid = $4
	`

	_, err := r.db.ExecContext(ctx, query, reaction, reactedAt, messageTrackID, reactorJID)
	if err != nil {
		return fmt.Errorf("failed to update message reaction: %w", err)
	}

	return nil
}

func (r *analyticsRepository) DeleteMessageReaction(ctx context.Context, messageTrackID, reactorJID string) error {
	query := `DELETE FROM message_reactions WHERE message_track_id = $1 AND reactor_jid = $2`

	_, err := r.db.ExecContext(ctx, query, messageTrackID, reactorJID)
	if err != nil {
		return fmt.Errorf("failed to delete message reaction: %w", err)
	}

	return nil
}

func (r *analyticsRepository) GetMessageReactions(ctx context.Context, messageTrackID string) ([]analytics.MessageReaction, error) {
	query := `
		SELECT id, message_track_id, reactor_jid, reactor_name, reaction, reacted_at, created_at
		FROM message_reactions
		WHERE message_track_id = $1
		ORDER BY reacted_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, messageTrackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message reactions: %w", err)
	}
	defer rows.Close()

	reactions := []analytics.MessageReaction{}
	for rows.Next() {
		var reaction analytics.MessageReaction
		err := rows.Scan(
			&reaction.ID, &reaction.MessageTrackID, &reaction.ReactorJID, &reaction.ReactorName,
			&reaction.Reaction, &reaction.ReactedAt, &reaction.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message reaction: %w", err)
		}
		reactions = append(reactions, reaction)
	}

	return reactions, nil
}

func (r *analyticsRepository) GetMessageMetrics(ctx context.Context, messageTrackID string) (*analytics.MessageMetrics, error) {
	tracking, err := r.GetMessageTrackingByID(ctx, messageTrackID)
	if err != nil {
		return nil, err
	}
	if tracking == nil {
		return nil, nil
	}

	views, err := r.GetMessageViews(ctx, messageTrackID)
	if err != nil {
		return nil, err
	}

	reactions, err := r.GetMessageReactions(ctx, messageTrackID)
	if err != nil {
		return nil, err
	}

	return &analytics.MessageMetrics{
		MessageTracking: *tracking,
		ViewCount:       len(views),
		ReactionCount:   len(reactions),
		Views:           views,
		Reactions:       reactions,
	}, nil
}

func (r *analyticsRepository) GetInstanceMessageMetrics(ctx context.Context, instanceID string, limit, offset int) ([]analytics.MessageMetricsSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT 
			mt.message_id,
			mt.remote_jid,
			mt.message_type,
			mt.sent_at,
			COUNT(DISTINCT mv.id) as view_count,
			COUNT(DISTINCT mr.id) as reaction_count
		FROM message_tracking mt
		LEFT JOIN message_views mv ON mt.id = mv.message_track_id
		LEFT JOIN message_reactions mr ON mt.id = mr.message_track_id
		WHERE mt.instance_id = $1
		GROUP BY mt.id, mt.message_id, mt.remote_jid, mt.message_type, mt.sent_at
		ORDER BY mt.sent_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, instanceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance message metrics: %w", err)
	}
	defer rows.Close()

	summaries := []analytics.MessageMetricsSummary{}
	for rows.Next() {
		var summary analytics.MessageMetricsSummary
		err := rows.Scan(
			&summary.MessageID, &summary.RemoteJID, &summary.MessageType, &summary.SentAt,
			&summary.ViewCount, &summary.ReactionCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message metrics summary: %w", err)
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}
