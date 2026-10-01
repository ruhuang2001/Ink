package workspace

import (
	"context"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
)

type Repository interface {
	FindByUserID(ctx context.Context, userID string) (*State, error)
	SaveByUserID(ctx context.Context, userID string, state State, updatedAt time.Time) error
}

type Authenticator interface {
	GetCurrentUser(ctx context.Context, accessToken string) (auth.UserDTO, error)
}

type Clock interface {
	Now() time.Time
}

type WorkspaceService interface {
	GetState(ctx context.Context, accessToken string) (State, error)
	SaveState(ctx context.Context, accessToken string, state State) (State, error)
}

type Service struct {
	repo  Repository
	auth  Authenticator
	clock Clock
}

func NewService(repo Repository, auth Authenticator, clock Clock) *Service {
	return &Service{
		repo:  repo,
		auth:  auth,
		clock: clock,
	}
}

func (s *Service) GetState(ctx context.Context, accessToken string) (State, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return State{}, err
	}

	current, err := s.repo.FindByUserID(ctx, currentUser.ID)
	if err != nil {
		return State{}, err
	}
	if current != nil {
		state := NormalizeState(*current)
		// Print history is owned by the printer repository and loaded through
		// its paginated API, rather than duplicated in workspace snapshots.
		state.PrintJobs = []PrintJob{}
		return state, nil
	}

	seeded := EmptyState()
	if err := s.repo.SaveByUserID(ctx, currentUser.ID, seeded, s.clock.Now()); err != nil {
		return State{}, err
	}

	return seeded, nil
}

func (s *Service) SaveState(ctx context.Context, accessToken string, state State) (State, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return State{}, err
	}

	normalized := NormalizeState(state)
	normalized.PrintJobs = []PrintJob{}
	if err := s.repo.SaveByUserID(ctx, currentUser.ID, normalized, s.clock.Now()); err != nil {
		return State{}, err
	}

	return normalized, nil
}

var _ WorkspaceService = (*Service)(nil)
