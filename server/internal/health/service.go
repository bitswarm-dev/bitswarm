package health

import (
	"context"

	"bitswarm/server/internal/logger"
)

type Service struct {
	repo Repository
	log  *logger.Logger
}

func NewService(repo Repository, log *logger.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) Check(ctx context.Context) (Check, error) {
	c, err := s.repo.Record(ctx)
	if err != nil {
		s.log.Error("failed to record health check", "error", err)
		return Check{}, err
	}
	return c, nil
}

func (s *Service) Latest(ctx context.Context) (Check, error) {
	return s.repo.Latest(ctx)
}
