package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"nusantara/internal/idgen"
	"nusantara/internal/store"
)

var (
	ErrNotStarted = errors.New("job service not started")
	ErrStopped    = errors.New("job service stopped")
)

type SiteProvisioner interface {
	ProvisionSite(ctx context.Context, site store.Site) error
	DeprovisionSite(ctx context.Context, site store.Site) error
}

type Service struct {
	repo            store.Repository
	logger          *log.Logger
	siteProvisioner SiteProvisioner
	queue           chan string
	started         bool
	stopped         bool
	mu              sync.RWMutex
	wg              sync.WaitGroup
	cancel          context.CancelFunc
}

func NewService(repo store.Repository, logger *log.Logger, siteProvisioner SiteProvisioner) *Service {
	return &Service{
		repo:            repo,
		logger:          logger,
		siteProvisioner: siteProvisioner,
		queue:           make(chan string, 256),
	}
}

func (s *Service) Start(parent context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.started = true
	s.wg.Add(1)
	go s.worker(ctx)
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) Enqueue(ctx context.Context, triggeredBy, jobType string, payload map[string]string) (store.Job, error) {
	s.mu.RLock()
	started := s.started
	stopped := s.stopped
	s.mu.RUnlock()
	if !started {
		return store.Job{}, ErrNotStarted
	}
	if stopped {
		return store.Job{}, ErrStopped
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return store.Job{}, err
	}
	jobID, err := idgen.New("job")
	if err != nil {
		return store.Job{}, err
	}

	now := time.Now().UTC()
	job := store.Job{
		ID:          jobID,
		Type:        jobType,
		Status:      store.JobStatusQueued,
		Payload:     string(body),
		CreatedAt:   now,
		TriggeredBy: triggeredBy,
	}
	if err := s.repo.CreateJob(ctx, job); err != nil {
		return store.Job{}, err
	}

	select {
	case s.queue <- job.ID:
	case <-ctx.Done():
		return store.Job{}, ctx.Err()
	}
	return job, nil
}

func (s *Service) List(ctx context.Context, limit int) ([]store.Job, error) {
	return s.repo.ListJobs(ctx, limit)
}

func (s *Service) Get(ctx context.Context, id string) (store.Job, error) {
	return s.repo.GetJobByID(ctx, id)
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case jobID := <-s.queue:
			s.handleJob(ctx, jobID)
		}
	}
}

func (s *Service) handleJob(ctx context.Context, jobID string) {
	job, err := s.repo.GetJobByID(ctx, jobID)
	if err != nil {
		s.logf("job load failed id=%s err=%v", jobID, err)
		return
	}

	startedAt := time.Now().UTC()
	if err := s.repo.UpdateJob(ctx, job.ID, store.JobStatusRunning, "", &startedAt, nil); err != nil {
		s.logf("job start failed id=%s err=%v", job.ID, err)
		return
	}

	runErr := s.runByType(ctx, job)
	finishedAt := time.Now().UTC()
	if runErr != nil {
		_ = s.repo.UpdateJob(ctx, job.ID, store.JobStatusFailed, runErr.Error(), &startedAt, &finishedAt)
		s.logf("job failed id=%s type=%s err=%v", job.ID, job.Type, runErr)
		return
	}
	if err := s.repo.UpdateJob(ctx, job.ID, store.JobStatusSuccess, "", &startedAt, &finishedAt); err != nil {
		s.logf("job finish write failed id=%s err=%v", job.ID, err)
	}
}

func (s *Service) runByType(ctx context.Context, job store.Job) error {
	switch job.Type {
	case store.JobTypeProvisionSite:
		return s.runProvisionSite(ctx, job)
	case store.JobTypeDeprovisionSite:
		return s.runDeprovisionSite(ctx, job)
	default:
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}
}

func (s *Service) runProvisionSite(ctx context.Context, job store.Job) error {
	if s.siteProvisioner == nil {
		return errors.New("site provisioner is not configured")
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	siteID := payload["site_id"]
	if siteID == "" {
		return errors.New("missing site_id in payload")
	}

	site, err := s.repo.GetSiteByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("load site: %w", err)
	}

	if err := s.siteProvisioner.ProvisionSite(ctx, site); err != nil {
		_ = s.repo.UpdateSiteStatus(ctx, site.ID, store.SiteStatusFailed)
		return err
	}

	if err := s.repo.UpdateSiteStatus(ctx, site.ID, store.SiteStatusActive); err != nil {
		return fmt.Errorf("update site status: %w", err)
	}
	return nil
}

func (s *Service) runDeprovisionSite(ctx context.Context, job store.Job) error {
	if s.siteProvisioner == nil {
		return errors.New("site provisioner is not configured")
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	siteID := payload["site_id"]
	if siteID == "" {
		return errors.New("missing site_id in payload")
	}

	site, err := s.repo.GetSiteByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("load site: %w", err)
	}

	if err := s.siteProvisioner.DeprovisionSite(ctx, site); err != nil {
		_ = s.repo.UpdateSiteStatus(ctx, site.ID, store.SiteStatusFailed)
		return err
	}

	if err := s.repo.DeleteSite(ctx, site.ID); err != nil {
		return fmt.Errorf("delete site metadata: %w", err)
	}
	return nil
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

