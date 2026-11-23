package service

import (
	"context"
	"fmt"

	"github.com/bidmybl/api-health-monitor/internal/features/checks"
	checkerRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

type EndpointRepository interface {
	GetEndpointByID(ctx context.Context, id int64) (endpointRepo.Endpoint, error)
}

type Checker interface {
	Check(ctx context.Context, rawURL string) checks.Result
}

type CheckRepository interface {
	Create(
		ctx context.Context,
		endpointID int64,
		result checks.Result,
	) (checkerRepo.CheckResult, error)
}

type Service struct {
	endpointRepo EndpointRepository
	checker      Checker
	checkRepo    CheckRepository
}

func NewService(
	endpointRepo EndpointRepository,
	checker Checker,
	checkRepo CheckRepository,
) *Service {
	return &Service{
		endpointRepo: endpointRepo,
		checker:      checker,
		checkRepo:    checkRepo,
	}
}

func (s *Service) CheckEndpoint(
	ctx context.Context,
	endpointID int64,
) (checkerRepo.CheckResult, error) {
	endpoint, err := s.endpointRepo.GetEndpointByID(ctx, endpointID)
	if err != nil {
		return checkerRepo.CheckResult{}, fmt.Errorf("get endpoint: %w", err)
	}

	result := s.checker.Check(ctx, endpoint.URL)

	savedResult, err := s.checkRepo.Create(ctx, endpoint.ID, result)
	if err != nil {
		return checkerRepo.CheckResult{}, fmt.Errorf("save check result: %w", err)
	}

	return savedResult, nil
}
