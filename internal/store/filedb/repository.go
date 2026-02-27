package filedb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nusantara/internal/store"
)

const schemaVersion = 1

type snapshot struct {
	SchemaVersion int                      `json:"schema_version"`
	Users         map[string]store.User    `json:"users"`
	Sessions      map[string]store.Session `json:"sessions"`
	Sites         map[string]store.Site    `json:"sites"`
	Jobs          map[string]store.Job     `json:"jobs"`
	AuditLogs     []store.AuditLog         `json:"audit_logs"`
	AuditSequence int64                    `json:"audit_sequence"`
	UsernameIndex map[string]string        `json:"username_index"`
	DomainIndex   map[string]string        `json:"domain_index"`
}

type Repository struct {
	mu   sync.RWMutex
	path string
	data snapshot
}

func New(path string) (*Repository, error) {
	if path == "" {
		return nil, errors.New("empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}

	repo := &Repository{
		path: path,
		data: newSnapshot(),
	}

	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

func newSnapshot() snapshot {
	return snapshot{
		SchemaVersion: schemaVersion,
		Users:         make(map[string]store.User),
		Sessions:      make(map[string]store.Session),
		Sites:         make(map[string]store.Site),
		Jobs:          make(map[string]store.Job),
		AuditLogs:     make([]store.AuditLog, 0, 128),
		UsernameIndex: make(map[string]string),
		DomainIndex:   make(map[string]string),
	}
}

func (r *Repository) load() error {
	raw, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r.save()
		}
		return fmt.Errorf("read state: %w", err)
	}

	var snap snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}
	if snap.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported schema version: %d", snap.SchemaVersion)
	}
	if snap.Users == nil {
		snap.Users = make(map[string]store.User)
	}
	if snap.Sessions == nil {
		snap.Sessions = make(map[string]store.Session)
	}
	if snap.Sites == nil {
		snap.Sites = make(map[string]store.Site)
	}
	if snap.Jobs == nil {
		snap.Jobs = make(map[string]store.Job)
	}
	if snap.AuditLogs == nil {
		snap.AuditLogs = make([]store.AuditLog, 0, 128)
	}
	if snap.UsernameIndex == nil {
		snap.UsernameIndex = make(map[string]string)
	}
	if snap.DomainIndex == nil {
		snap.DomainIndex = make(map[string]string)
	}

	r.data = snap
	r.rebuildIndexes()
	return nil
}

func (r *Repository) rebuildIndexes() {
	r.data.UsernameIndex = make(map[string]string, len(r.data.Users))
	for id, user := range r.data.Users {
		r.data.UsernameIndex[strings.ToLower(user.Username)] = id
	}

	r.data.DomainIndex = make(map[string]string, len(r.data.Sites))
	for id, site := range r.data.Sites {
		r.data.DomainIndex[strings.ToLower(site.Domain)] = id
	}
}

func (r *Repository) save() error {
	raw, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	tmpPath := r.path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o640); err != nil {
		return fmt.Errorf("write tmp state: %w", err)
	}
	if err := os.Rename(tmpPath, r.path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

func (r *Repository) Migrate(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data.SchemaVersion == 0 {
		r.data.SchemaVersion = schemaVersion
	}
	return r.save()
}

func (r *Repository) CountUsers(_ context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.data.Users), nil
}

func (r *Repository) CreateUser(_ context.Context, user store.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	usernameKey := strings.ToLower(user.Username)
	if _, exists := r.data.UsernameIndex[usernameKey]; exists {
		return store.ErrConflict
	}

	r.data.Users[user.ID] = user
	r.data.UsernameIndex[usernameKey] = user.ID
	return r.save()
}

func (r *Repository) GetUserByUsername(_ context.Context, username string) (store.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.data.UsernameIndex[strings.ToLower(username)]
	if !ok {
		return store.User{}, store.ErrNotFound
	}

	user, ok := r.data.Users[id]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return user, nil
}

func (r *Repository) GetUserByID(_ context.Context, id string) (store.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.data.Users[id]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return user, nil
}

func (r *Repository) UpdateUserPassword(_ context.Context, id, passwordHash string, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.data.Users[id]
	if !ok {
		return store.ErrNotFound
	}
	user.PasswordHash = passwordHash
	user.UpdatedAt = updatedAt
	r.data.Users[id] = user
	return r.save()
}

func (r *Repository) CreateSession(_ context.Context, session store.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data.Sessions[session.TokenHash] = session
	return r.save()
}

func (r *Repository) GetSessionByTokenHash(_ context.Context, tokenHash string) (store.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.data.Sessions[tokenHash]
	if !ok {
		return store.Session{}, store.ErrNotFound
	}
	return session, nil
}

func (r *Repository) DeleteSessionByTokenHash(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data.Sessions, tokenHash)
	return r.save()
}

func (r *Repository) CreateSite(_ context.Context, site store.Site) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	domainKey := strings.ToLower(site.Domain)
	if _, exists := r.data.DomainIndex[domainKey]; exists {
		return store.ErrConflict
	}

	r.data.Sites[site.ID] = site
	r.data.DomainIndex[domainKey] = site.ID
	return r.save()
}

func (r *Repository) ListSites(_ context.Context, limit int) ([]store.Site, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sites := make([]store.Site, 0, len(r.data.Sites))
	for _, site := range r.data.Sites {
		sites = append(sites, site)
	}
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].CreatedAt.After(sites[j].CreatedAt)
	})
	if limit > 0 && len(sites) > limit {
		sites = sites[:limit]
	}
	return sites, nil
}

func (r *Repository) GetSiteByID(_ context.Context, id string) (store.Site, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	site, ok := r.data.Sites[id]
	if !ok {
		return store.Site{}, store.ErrNotFound
	}
	return site, nil
}

func (r *Repository) UpdateSiteStatus(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	site, ok := r.data.Sites[id]
	if !ok {
		return store.ErrNotFound
	}
	site.Status = status
	site.UpdatedAt = time.Now().UTC()
	r.data.Sites[id] = site
	return r.save()
}

func (r *Repository) DeleteSite(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	site, ok := r.data.Sites[id]
	if !ok {
		return store.ErrNotFound
	}
	delete(r.data.DomainIndex, strings.ToLower(site.Domain))
	delete(r.data.Sites, id)
	return r.save()
}

func (r *Repository) CreateJob(_ context.Context, job store.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data.Jobs[job.ID] = job
	return r.save()
}

func (r *Repository) ListJobs(_ context.Context, limit int) ([]store.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	jobs := make([]store.Job, 0, len(r.data.Jobs))
	for _, job := range r.data.Jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func (r *Repository) GetJobByID(_ context.Context, id string) (store.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.data.Jobs[id]
	if !ok {
		return store.Job{}, store.ErrNotFound
	}
	return job, nil
}

func (r *Repository) UpdateJob(_ context.Context, id, status, errorMsg string, startedAt, finishedAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.data.Jobs[id]
	if !ok {
		return store.ErrNotFound
	}
	job.Status = status
	job.Error = errorMsg
	job.StartedAt = startedAt
	job.FinishedAt = finishedAt
	r.data.Jobs[id] = job
	return r.save()
}

func (r *Repository) CreateAuditLog(_ context.Context, logEntry store.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data.AuditSequence++
	logEntry.ID = r.data.AuditSequence
	r.data.AuditLogs = append(r.data.AuditLogs, logEntry)
	return r.save()
}

func (r *Repository) ListAuditLogs(_ context.Context, limit int) ([]store.AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	size := len(r.data.AuditLogs)
	if limit <= 0 || limit > size {
		limit = size
	}
	out := make([]store.AuditLog, 0, limit)
	for i := size - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.data.AuditLogs[i])
	}
	return out, nil
}

func (r *Repository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.save()
}

