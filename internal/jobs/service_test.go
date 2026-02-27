package jobs

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"testing"
	"time"

	"nusantara/internal/store"
	"nusantara/internal/store/filedb"
)

type fakeProvisioner struct {
	err              error
	provisionCount   int
	deprovisionCount int
}

func (f *fakeProvisioner) ProvisionSite(_ context.Context, _ store.Site) error {
	f.provisionCount++
	return f.err
}

func (f *fakeProvisioner) DeprovisionSite(_ context.Context, _ store.Site) error {
	f.deprovisionCount++
	return f.err
}

func TestServiceRunsProvisionJob(t *testing.T) {
	repo, err := filedb.New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	if err := repo.CreateSite(ctx, store.Site{
		ID:        "site-1",
		Domain:    "example.com",
		RootPath:  "/var/www/example",
		Runtime:   "php",
		Status:    store.SiteStatusProvisioning,
		CreatedBy: "usr-1",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	svc := NewService(repo, log.New(testWriter{t}, "", 0), &fakeProvisioner{})
	svc.Start(ctx)
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = svc.Stop(stopCtx)
	}()

	job, err := svc.Enqueue(ctx, "usr-1", store.JobTypeProvisionSite, map[string]string{
		"site_id": "site-1",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	var final store.Job
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		final, err = svc.Get(ctx, job.ID)
		if err == nil && (final.Status == store.JobStatusSuccess || final.Status == store.JobStatusFailed) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if final.Status != store.JobStatusSuccess {
		t.Fatalf("job status = %s err=%s", final.Status, final.Error)
	}

	site, err := repo.GetSiteByID(ctx, "site-1")
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if site.Status != store.SiteStatusActive {
		t.Fatalf("site status = %s", site.Status)
	}
}

func TestServiceRunsDeprovisionJob(t *testing.T) {
	repo, err := filedb.New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	if err := repo.CreateSite(ctx, store.Site{
		ID:        "site-2",
		Domain:    "remove.example.com",
		RootPath:  "/var/www/remove",
		Runtime:   "php",
		Status:    store.SiteStatusActive,
		CreatedBy: "usr-1",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create site: %v", err)
	}

	fake := &fakeProvisioner{}
	svc := NewService(repo, nil, fake)
	svc.Start(ctx)
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = svc.Stop(stopCtx)
	}()

	job, err := svc.Enqueue(ctx, "usr-1", store.JobTypeDeprovisionSite, map[string]string{
		"site_id": "site-2",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		final, _ := svc.Get(ctx, job.ID)
		if final.Status == store.JobStatusSuccess {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if fake.deprovisionCount != 1 {
		t.Fatalf("expected deprovision called once, got %d", fake.deprovisionCount)
	}
	if _, err := repo.GetSiteByID(ctx, "site-2"); err == nil {
		t.Fatalf("expected site metadata deleted")
	}
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", string(p))
	return len(p), nil
}

func TestServiceHandlesBadPayload(t *testing.T) {
	repo, err := filedb.New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	defer repo.Close()

	now := time.Now().UTC()
	job := store.Job{
		ID:          "job-1",
		Type:        store.JobTypeProvisionSite,
		Status:      store.JobStatusQueued,
		Payload:     "{}",
		CreatedAt:   now,
		TriggeredBy: "usr-1",
	}
	if err := repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	svc := NewService(repo, nil, &fakeProvisioner{})
	svc.Start(context.Background())
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = svc.Stop(stopCtx)
	}()

	svc.queue <- job.ID

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := repo.GetJobByID(context.Background(), job.ID)
		if got.Status == store.JobStatusFailed {
			if got.Error == "" {
				t.Fatalf("expected error message")
			}
			var payload map[string]string
			_ = json.Unmarshal([]byte(got.Payload), &payload)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("job did not transition to failed")
}

