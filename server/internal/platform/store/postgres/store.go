package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/session"
	"github.com/ruhuang/ink/server/internal/user"
	"github.com/ruhuang/ink/server/internal/workspace"
)

// Store implements the auth repositories on top of PostgreSQL.
type Store struct {
	db *pgxpool.Pool
}

var (
	_ auth.UserRepository    = (*Store)(nil)
	_ auth.SessionRepository = (*Store)(nil)
	_ auth.AuditLogger       = (*Store)(nil)
	_ workspace.Repository   = (*Store)(nil)
	_ ai.Repository          = (*Store)(nil)
	_ printer.Repository     = (*Store)(nil)
)

// New creates a PostgreSQL-backed auth store.
func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// FindByEmail loads a user by email address.
func (s *Store) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	row := s.db.QueryRow(ctx, `
		select id, email, password_hash, display_name, role, status, created_at, updated_at, last_login_at
		from users
		where email = $1
	`, email)

	return scanUser(row)
}

// FindUserByID loads a user by identifier.
func (s *Store) FindUserByID(ctx context.Context, id string) (*user.User, error) {
	row := s.db.QueryRow(ctx, `
		select id, email, password_hash, display_name, role, status, created_at, updated_at, last_login_at
		from users
		where id = $1
	`, id)

	return scanUser(row)
}

// FindPrimaryAdmin loads the oldest active administrator account.
func (s *Store) FindPrimaryAdmin(ctx context.Context) (*user.User, error) {
	row := s.db.QueryRow(ctx, `
		select id, email, password_hash, display_name, role, status, created_at, updated_at, last_login_at
		from users
		where role = 'admin' and status = 'active'
		order by created_at asc
		limit 1
	`)

	return scanUser(row)
}

// CreateUser inserts a new user record.
func (s *Store) CreateUser(ctx context.Context, current user.User) error {
	_, err := s.db.Exec(ctx, `
		insert into users (
			id, email, password_hash, display_name, role, status, created_at, updated_at, last_login_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, null)
	`,
		current.ID,
		current.Email,
		current.PasswordHash,
		current.DisplayName,
		current.Role,
		current.Status,
		current.CreatedAt,
		current.UpdatedAt,
	)
	return err
}

// UpdateLastLoginAt stores the latest successful login time for a user.
func (s *Store) UpdateLastLoginAt(ctx context.Context, userID string, at time.Time) error {
	_, err := s.db.Exec(ctx, `update users set last_login_at = $2, updated_at = $2 where id = $1`, userID, at)
	return err
}

// UpdatePasswordHash replaces the stored password digest for a user.
func (s *Store) UpdatePasswordHash(
	ctx context.Context,
	userID string,
	passwordHash string,
	at time.Time,
) error {
	_, err := s.db.Exec(ctx, `
		update users
		set password_hash = $2, updated_at = $3
		where id = $1
	`, userID, passwordHash, at)
	return err
}

// Create inserts a new auth session row.
func (s *Store) Create(ctx context.Context, current session.Session) error {
	_, err := s.db.Exec(ctx, `
		insert into auth_sessions (
			id, family_id, user_id, refresh_token_hash, client_type, user_agent, ip_address,
			expires_at, rotated_at, revoked_at, created_at, last_used_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, null, null, $9, $10)
	`,
		current.ID,
		current.FamilyID,
		current.UserID,
		current.RefreshTokenHash,
		current.ClientType,
		nullableString(current.UserAgent),
		nullableString(current.IPAddress),
		current.ExpiresAt,
		current.CreatedAt,
		current.LastUsedAt,
	)
	return err
}

// FindByRefreshTokenHash loads a session by refresh-token digest.
func (s *Store) FindByRefreshTokenHash(ctx context.Context, hash string) (*session.Session, error) {
	row := s.db.QueryRow(ctx, `
		select id, family_id, user_id, refresh_token_hash, client_type,
			coalesce(user_agent, ''), coalesce(ip_address, ''),
			expires_at, rotated_at, revoked_at, created_at, last_used_at
		from auth_sessions
		where refresh_token_hash = $1
	`, hash)

	return scanSession(row)
}

// FindSessionByID loads a session by identifier.
func (s *Store) FindSessionByID(ctx context.Context, id string) (*session.Session, error) {
	row := s.db.QueryRow(ctx, `
		select id, family_id, user_id, refresh_token_hash, client_type,
			coalesce(user_agent, ''), coalesce(ip_address, ''),
			expires_at, rotated_at, revoked_at, created_at, last_used_at
		from auth_sessions
		where id = $1
	`, id)

	return scanSession(row)
}

// Rotate marks the current session as rotated and inserts the replacement session atomically.
func (s *Store) Rotate(
	ctx context.Context,
	current session.Session,
	next session.Session,
	rotatedAt time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tag, err := tx.Exec(ctx, `
		update auth_sessions
		set rotated_at = $2, last_used_at = $2
		where id = $1 and rotated_at is null and revoked_at is null
	`, current.ID, rotatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("refresh session %s was already rotated or revoked", current.ID)
	}

	if _, err := tx.Exec(ctx, `
		insert into auth_sessions (
			id, family_id, user_id, refresh_token_hash, client_type, user_agent, ip_address,
			expires_at, rotated_at, revoked_at, created_at, last_used_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, null, null, $9, $10)
	`,
		next.ID,
		next.FamilyID,
		next.UserID,
		next.RefreshTokenHash,
		next.ClientType,
		nullableString(next.UserAgent),
		nullableString(next.IPAddress),
		next.ExpiresAt,
		next.CreatedAt,
		next.LastUsedAt,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RevokeFamily revokes every session in the same refresh-token family.
func (s *Store) RevokeFamily(ctx context.Context, familyID string, revokedAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, $2)
		where family_id = $1
	`, familyID, revokedAt)
	return err
}

// RevokeByID revokes a single session by identifier.
func (s *Store) RevokeByID(ctx context.Context, sessionID string, revokedAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, $2)
		where id = $1
	`, sessionID, revokedAt)
	return err
}

// RevokeByUserID revokes all sessions belonging to a user.
func (s *Store) RevokeByUserID(ctx context.Context, userID string, revokedAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, $2)
		where user_id = $1
	`, userID, revokedAt)
	return err
}

// Log persists an authentication audit event.
func (s *Store) Log(ctx context.Context, event auth.AuditEvent) error {
	var detail any
	if event.Detail != nil {
		payload, err := json.Marshal(event.Detail)
		if err != nil {
			return err
		}
		detail = payload
	}

	_, err := s.db.Exec(ctx, `
		insert into auth_audit_logs (
			id, user_id, event_type, client_type, ip_address, user_agent, detail, created_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		fmt.Sprintf("al_%d", time.Now().UnixNano()),
		event.UserID,
		event.EventType,
		event.ClientType,
		nullableString(event.IPAddress),
		nullableString(event.UserAgent),
		detail,
		event.CreatedAt,
	)
	return err
}

// FindByUserID loads a persisted workspace snapshot for a user.
func (s *Store) FindByUserID(ctx context.Context, userID string) (*workspace.State, error) {
	row := s.db.QueryRow(ctx, `
		select state - 'printJobs'
		from workspace_snapshots
		where user_id = $1
	`, userID)

	var payload []byte
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var current workspace.State
	if err := json.Unmarshal(payload, &current); err != nil {
		return nil, err
	}

	return &current, nil
}

// SaveByUserID upserts the current workspace snapshot for a user.
func (s *Store) SaveByUserID(
	ctx context.Context,
	userID string,
	state workspace.State,
	updatedAt time.Time,
) error {
	state.PrintJobs = []workspace.PrintJob{}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx, `
		insert into workspace_snapshots (user_id, state, created_at, updated_at)
		values ($1, $2, $3, $3)
		on conflict (user_id)
		do update set state = excluded.state, updated_at = excluded.updated_at
	`, userID, payload, updatedAt)
	return err
}

func (s *Store) GetSystemConfig(ctx context.Context) (*ai.StoredConfig, error) {
	row := s.db.QueryRow(ctx, `
		select provider_name, provider_type, base_url, model, api_key_ciphertext, api_key_nonce,
			updated_by, created_at, updated_at
		from ai_provider_settings
		where setting_key = 'system'
	`)

	var config ai.StoredConfig
	var updatedBy *string
	if err := row.Scan(
		&config.ProviderName,
		&config.ProviderType,
		&config.BaseURL,
		&config.Model,
		&config.Ciphertext,
		&config.Nonce,
		&updatedBy,
		&config.CreatedAt,
		&config.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	config.UpdatedByUser = updatedBy
	return &config, nil
}

func (s *Store) SaveSystemConfig(ctx context.Context, config ai.StoredConfig) error {
	_, err := s.db.Exec(ctx, `
		insert into ai_provider_settings (
			setting_key, provider_name, provider_type, base_url, model,
			api_key_ciphertext, api_key_nonce, updated_by, created_at, updated_at
		) values ('system', $1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (setting_key)
		do update set
			provider_name = excluded.provider_name,
			provider_type = excluded.provider_type,
			base_url = excluded.base_url,
			model = excluded.model,
			api_key_ciphertext = excluded.api_key_ciphertext,
			api_key_nonce = excluded.api_key_nonce,
			updated_by = excluded.updated_by,
			updated_at = excluded.updated_at
	`,
		config.ProviderName,
		config.ProviderType,
		config.BaseURL,
		config.Model,
		config.Ciphertext,
		config.Nonce,
		config.UpdatedByUser,
		config.CreatedAt,
		config.UpdatedAt,
	)
	return err
}

func (s *Store) ListBindingsByUserID(ctx context.Context, userID string) ([]printer.Binding, error) {
	rows, err := s.db.Query(ctx, `
		select id, user_id, name, note, device_identifier, provider_user_id, status, created_at, updated_at
		from printer_bindings
		where user_id = $1
		order by updated_at desc, created_at desc
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bindings []printer.Binding
	for rows.Next() {
		var binding printer.Binding
		if err := rows.Scan(
			&binding.ID,
			&binding.UserID,
			&binding.Name,
			&binding.Note,
			&binding.DeviceIdentifier,
			&binding.ProviderUserID,
			&binding.Status,
			&binding.CreatedAt,
			&binding.UpdatedAt,
		); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}

	return bindings, rows.Err()
}

func (s *Store) FindBindingByID(ctx context.Context, userID string, bindingID string) (*printer.Binding, error) {
	row := s.db.QueryRow(ctx, `
		select id, user_id, name, note, device_identifier, provider_user_id, status, created_at, updated_at
		from printer_bindings
		where user_id = $1 and id = $2
	`, userID, bindingID)

	var binding printer.Binding
	if err := row.Scan(
		&binding.ID,
		&binding.UserID,
		&binding.Name,
		&binding.Note,
		&binding.DeviceIdentifier,
		&binding.ProviderUserID,
		&binding.Status,
		&binding.CreatedAt,
		&binding.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &binding, nil
}

func (s *Store) SaveBinding(ctx context.Context, binding printer.Binding) error {
	_, err := s.db.Exec(ctx, `
		insert into printer_bindings (
			id, user_id, name, note, device_identifier, provider_user_id, status, created_at, updated_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (id)
		do update set
			name = excluded.name,
			note = excluded.note,
			device_identifier = excluded.device_identifier,
			provider_user_id = excluded.provider_user_id,
			status = excluded.status,
			updated_at = excluded.updated_at
	`,
		binding.ID,
		binding.UserID,
		binding.Name,
		binding.Note,
		binding.DeviceIdentifier,
		binding.ProviderUserID,
		binding.Status,
		binding.CreatedAt,
		binding.UpdatedAt,
	)
	return err
}

func (s *Store) DeleteBinding(ctx context.Context, userID string, bindingID string) error {
	_, err := s.db.Exec(ctx, `
		delete from printer_bindings
		where user_id = $1 and id = $2
	`, userID, bindingID)
	return err
}

func (s *Store) FindJobByID(ctx context.Context, userID string, jobID string) (*printer.Job, error) {
	row := s.db.QueryRow(ctx, `
		select id, user_id, printer_binding_id, title, source, content, status,
			provider_print_content_id, provider_smart_guid, error_message, created_at, updated_at, next_status_check_at, status_check_attempts
		from print_jobs
		where user_id = $1 and id = $2
	`, userID, jobID)

	var job printer.Job
	var providerPrintContentID *int
	var providerSmartGUID *string
	var errorMessage *string
	if err := row.Scan(
		&job.ID,
		&job.UserID,
		&job.PrinterBindingID,
		&job.Title,
		&job.Source,
		&job.Content,
		&job.Status,
		&providerPrintContentID,
		&providerSmartGUID,
		&errorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.NextStatusCheckAt,
		&job.StatusCheckAttempts,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	job.ProviderPrintContentID = providerPrintContentID
	job.ProviderSmartGUID = providerSmartGUID
	job.ErrorMessage = errorMessage
	return &job, nil
}

func (s *Store) SaveJob(ctx context.Context, job printer.Job) error {
	_, err := s.db.Exec(ctx, `
		insert into print_jobs (
			id, user_id, printer_binding_id, title, source, content, status,
			provider_print_content_id, provider_smart_guid, error_message, created_at, updated_at, next_status_check_at, status_check_attempts
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		on conflict (id)
		do update set
			printer_binding_id = excluded.printer_binding_id,
			title = excluded.title,
			source = excluded.source,
			content = excluded.content,
			status = excluded.status,
			provider_print_content_id = excluded.provider_print_content_id,
			provider_smart_guid = excluded.provider_smart_guid,
			error_message = excluded.error_message,
			updated_at = excluded.updated_at,
			next_status_check_at = excluded.next_status_check_at,
			status_check_attempts = excluded.status_check_attempts
	`,
		job.ID,
		job.UserID,
		job.PrinterBindingID,
		job.Title,
		job.Source,
		job.Content,
		job.Status,
		job.ProviderPrintContentID,
		job.ProviderSmartGUID,
		job.ErrorMessage,
		job.CreatedAt,
		job.UpdatedAt,
		job.NextStatusCheckAt,
		job.StatusCheckAttempts,
	)
	return err
}

func (s *Store) SavePendingJob(ctx context.Context, job printer.Job, expectedUpdatedAt time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		update print_jobs set
			printer_binding_id = $4,
			status = $5,
			error_message = $6,
			updated_at = $7,
			next_status_check_at = $8,
			status_check_attempts = $9
		where id = $1 and user_id = $2 and status = 'pending' and updated_at = $3
	`,
		job.ID,
		job.UserID,
		expectedUpdatedAt,
		job.PrinterBindingID,
		job.Status,
		job.ErrorMessage,
		job.UpdatedAt,
		job.NextStatusCheckAt,
		job.StatusCheckAttempts,
	)
	return tag.RowsAffected() == 1, err
}

func scanUser(row pgx.Row) (*user.User, error) {
	var account user.User
	var lastLoginAt *time.Time
	if err := row.Scan(
		&account.ID,
		&account.Email,
		&account.PasswordHash,
		&account.DisplayName,
		&account.Role,
		&account.Status,
		&account.CreatedAt,
		&account.UpdatedAt,
		&lastLoginAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	account.LastLoginAt = lastLoginAt
	return &account, nil
}

func scanSession(row pgx.Row) (*session.Session, error) {
	var current session.Session
	if err := row.Scan(
		&current.ID,
		&current.FamilyID,
		&current.UserID,
		&current.RefreshTokenHash,
		&current.ClientType,
		&current.UserAgent,
		&current.IPAddress,
		&current.ExpiresAt,
		&current.RotatedAt,
		&current.RevokedAt,
		&current.CreatedAt,
		&current.LastUsedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &current, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}

	return value
}
