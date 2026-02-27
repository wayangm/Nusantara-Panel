package store

import (
	"context"
	"errors"
	"time"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	SiteStatusProvisioning = "provisioning"
	SiteStatusActive       = "active"
	SiteStatusFailed       = "failed"
	SiteStatusDeleting     = "deleting"

	JobStatusQueued  = "queued"
	JobStatusRunning = "running"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"

	JobTypeProvisionSite   = "provision_site"
	JobTypeDeprovisionSite = "deprovision_site"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	TokenHash string    `json:"token_hash"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Site struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	RootPath  string    `json:"root_path"`
	Runtime   string    `json:"runtime"`
	Status    string    `json:"status"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Job struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Payload     string     `json:"payload"`
	Error       string     `json:"error"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	TriggeredBy string     `json:"triggered_by"`
}

type AuditLog struct {
	ID         int64     `json:"id"`
	ActorUser  string    `json:"actor_user"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Metadata   string    `json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
}

type Repository interface {
	Migrate(ctx context.Context) error

	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, user User) error
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	UpdateUserPassword(ctx context.Context, id, passwordHash string, updatedAt time.Time) error

	CreateSession(ctx context.Context, session Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error

	CreateSite(ctx context.Context, site Site) error
	ListSites(ctx context.Context, limit int) ([]Site, error)
	GetSiteByID(ctx context.Context, id string) (Site, error)
	UpdateSiteStatus(ctx context.Context, id, status string) error
	DeleteSite(ctx context.Context, id string) error

	CreateJob(ctx context.Context, job Job) error
	ListJobs(ctx context.Context, limit int) ([]Job, error)
	GetJobByID(ctx context.Context, id string) (Job, error)
	UpdateJob(ctx context.Context, id, status, errorMsg string, startedAt, finishedAt *time.Time) error

	CreateAuditLog(ctx context.Context, log AuditLog) error
	ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error)

	Close() error
}

