package audit

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"nusantara/internal/store"
)

type Service struct {
	repo   store.Repository
	logger *log.Logger
}

func NewService(repo store.Repository, logger *log.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) Record(ctx context.Context, actorUserID, action, targetType, targetID string, metadata map[string]any) {
	body, err := json.Marshal(metadata)
	if err != nil {
		body = []byte("{}")
	}
	entry := store.AuditLog{
		ActorUser:  actorUserID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Metadata:   string(body),
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.repo.CreateAuditLog(ctx, entry); err != nil && s.logger != nil {
		s.logger.Printf("audit write failed action=%s target=%s err=%v", action, targetID, err)
	}
}

func (s *Service) List(ctx context.Context, limit int) ([]store.AuditLog, error) {
	return s.repo.ListAuditLogs(ctx, limit)
}

