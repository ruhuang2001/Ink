package workspace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
)

var (
	ErrConflict         = errors.New("workspace revision conflict")
	ErrRevisionRequired = errors.New("workspace revision required")
)

type Repository interface {
	FindByUserID(ctx context.Context, userID string) (*State, error)
	InitializeByUserID(ctx context.Context, userID string, state State, createdAt time.Time) error
	SaveByUserID(ctx context.Context, userID string, state State, updatedAt time.Time) (int64, error)
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
	if current == nil {
		if err := s.repo.InitializeByUserID(ctx, currentUser.ID, EmptyState(), s.clock.Now()); err != nil {
			return State{}, err
		}
		current, err = s.repo.FindByUserID(ctx, currentUser.ID)
		if err != nil {
			return State{}, err
		}
		if current == nil {
			return State{}, fmt.Errorf("initialized workspace %s not found", currentUser.ID)
		}
	}

	state := NormalizeState(*current)
	// Print history is owned by the printer repository and loaded through
	// its paginated API, rather than duplicated in workspace snapshots.
	state.PrintJobs = []PrintJob{}
	return state, nil
}

func (s *Service) SaveState(ctx context.Context, accessToken string, state State) (State, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return State{}, err
	}
	if state.Revision < 1 {
		return State{}, ErrRevisionRequired
	}

	normalized := NormalizeState(state)
	normalized.PrintJobs = []PrintJob{}
	revision, err := s.repo.SaveByUserID(ctx, currentUser.ID, normalized, s.clock.Now())
	if err != nil {
		return State{}, err
	}
	normalized.Revision = revision

	return normalized, nil
}

var _ WorkspaceService = (*Service)(nil)
