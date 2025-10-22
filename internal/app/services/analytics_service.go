package services

import (
	"context"
	"fmt"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/domain/analytics"
	"github.com/faeln1/go-whatsapp-api/internal/domain/message"
)

type AnalyticsService interface {
	TrackSentMessage(ctx context.Context, instanceID, communityJID string, msg message.SendTextOutput, content, mediaURL, caption string) (*analytics.MessageTracking, error)
	RecordMessageView(ctx context.Context, messageID, viewerJID, viewerName string, viewedAt time.Time) error
	RecordMessageReaction(ctx context.Context, messageID, reactorJID, reactorName, reaction string) error
	GetMessageMetrics(ctx context.Context, messageTrackID string) (*analytics.MessageMetrics, error)
	GetInstanceMetrics(ctx context.Context, instanceID string, limit, offset int) ([]analytics.MessageMetricsSummary, error)
}

type analyticsService struct {
	repo repositories.AnalyticsRepository
}

func NewAnalyticsService(repo repositories.AnalyticsRepository) AnalyticsService {
	return &analyticsService{repo: repo}
}

func (s *analyticsService) TrackSentMessage(ctx context.Context, instanceID, communityJID string, msg message.SendTextOutput, content, mediaURL, caption string) (*analytics.MessageTracking, error) {
	input := analytics.CreateMessageTrackingInput{
		InstanceID:   instanceID,
		MessageID:    msg.Key.ID,
		RemoteJID:    msg.Key.RemoteJID,
		CommunityJID: communityJID,
		MessageType:  msg.MessageType,
		Content:      content,
		MediaURL:     mediaURL,
		Caption:      caption,
		SentAt:       time.Unix(msg.MessageTimestamp, 0),
	}

	return s.repo.CreateMessageTracking(ctx, input)
}

func (s *analyticsService) RecordMessageView(ctx context.Context, messageID, viewerJID, viewerName string, viewedAt time.Time) error {
	// Buscar o tracking da mensagem
	tracking, err := s.repo.GetMessageTracking(ctx, messageID)
	if err != nil {
		return fmt.Errorf("failed to get message tracking: %w", err)
	}
	if tracking == nil {
		// Mensagem não está sendo rastreada, ignorar
		return nil
	}

	if viewedAt.IsZero() {
		viewedAt = time.Now().UTC()
	}

	input := analytics.CreateMessageViewInput{
		MessageTrackID: tracking.ID,
		ViewerJID:      viewerJID,
		ViewerName:     viewerName,
		ViewedAt:       viewedAt,
	}

	_, err = s.repo.CreateMessageView(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to record message view: %w", err)
	}

	return nil
}

func (s *analyticsService) RecordMessageReaction(ctx context.Context, messageID, reactorJID, reactorName, reaction string) error {
	// Buscar o tracking da mensagem
	tracking, err := s.repo.GetMessageTracking(ctx, messageID)
	if err != nil {
		return fmt.Errorf("failed to get message tracking: %w", err)
	}
	if tracking == nil {
		// Mensagem não está sendo rastreada, ignorar
		return nil
	}

	// Se reaction está vazio, deletar a reação
	if reaction == "" {
		return s.repo.DeleteMessageReaction(ctx, tracking.ID, reactorJID)
	}

	// Tentar atualizar reação existente
	err = s.repo.UpdateMessageReaction(ctx, tracking.ID, reactorJID, reaction, time.Now())
	if err != nil {
		// Se não existe, criar nova
		input := analytics.CreateMessageReactionInput{
			MessageTrackID: tracking.ID,
			ReactorJID:     reactorJID,
			ReactorName:    reactorName,
			Reaction:       reaction,
			ReactedAt:      time.Now(),
		}

		_, err = s.repo.CreateMessageReaction(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to record message reaction: %w", err)
		}
	}

	return nil
}

func (s *analyticsService) GetMessageMetrics(ctx context.Context, messageTrackID string) (*analytics.MessageMetrics, error) {
	return s.repo.GetMessageMetrics(ctx, messageTrackID)
}

func (s *analyticsService) GetInstanceMetrics(ctx context.Context, instanceID string, limit, offset int) ([]analytics.MessageMetricsSummary, error) {
	return s.repo.GetInstanceMessageMetrics(ctx, instanceID, limit, offset)
}
