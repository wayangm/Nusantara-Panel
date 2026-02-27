package sites

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"nusantara/internal/idgen"
	"nusantara/internal/jobs"
	"nusantara/internal/store"
)

var (
	ErrInvalidDomain  = errors.New("invalid domain")
	ErrInvalidRoot    = errors.New("invalid root_path")
	ErrInvalidRuntime = errors.New("invalid runtime")
	ErrInvalidFile    = errors.New("invalid file")
	ErrContentTooLong = errors.New("content too long")
)

var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

var allowedRuntime = map[string]struct{}{
	"php":    {},
	"node":   {},
	"python": {},
	"static": {},
}

var editableFiles = map[string]struct{}{
	"index.html": {},
	"index.htm":  {},
	"index.php":  {},
}

const maxSiteContentBytes = 1024 * 1024 // 1 MiB

type Service struct {
	repo   store.Repository
	jobSvc *jobs.Service
}

type CreateSiteInput struct {
	Domain   string
	RootPath string
	Runtime  string
}

func NewService(repo store.Repository, jobSvc *jobs.Service) *Service {
	return &Service{
		repo:   repo,
		jobSvc: jobSvc,
	}
}

func (s *Service) CreateSite(ctx context.Context, actorID string, input CreateSiteInput) (store.Site, store.Job, error) {
	domain := normalizeDomain(input.Domain)
	if !isValidDomain(domain) {
		return store.Site{}, store.Job{}, ErrInvalidDomain
	}

	rootPath := strings.TrimSpace(input.RootPath)
	if !isValidRootPath(rootPath) {
		return store.Site{}, store.Job{}, ErrInvalidRoot
	}

	runtime := strings.ToLower(strings.TrimSpace(input.Runtime))
	if _, ok := allowedRuntime[runtime]; !ok {
		return store.Site{}, store.Job{}, ErrInvalidRuntime
	}

	siteID, err := idgen.New("site")
	if err != nil {
		return store.Site{}, store.Job{}, err
	}
	now := time.Now().UTC()
	site := store.Site{
		ID:        siteID,
		Domain:    domain,
		RootPath:  rootPath,
		Runtime:   runtime,
		Status:    store.SiteStatusProvisioning,
		CreatedBy: actorID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateSite(ctx, site); err != nil {
		return store.Site{}, store.Job{}, err
	}

	job, err := s.jobSvc.Enqueue(ctx, actorID, store.JobTypeProvisionSite, map[string]string{
		"site_id":  site.ID,
		"domain":   site.Domain,
		"runtime":  site.Runtime,
		"rootPath": site.RootPath,
	})
	if err != nil {
		_ = s.repo.UpdateSiteStatus(ctx, site.ID, store.SiteStatusFailed)
		return store.Site{}, store.Job{}, err
	}
	return site, job, nil
}

func (s *Service) ListSites(ctx context.Context, limit int) ([]store.Site, error) {
	return s.repo.ListSites(ctx, limit)
}

func (s *Service) GetSite(ctx context.Context, id string) (store.Site, error) {
	return s.repo.GetSiteByID(ctx, id)
}

func (s *Service) DeleteSite(ctx context.Context, actorID, id string) (store.Job, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Job{}, err
	}
	if err := s.repo.UpdateSiteStatus(ctx, site.ID, store.SiteStatusDeleting); err != nil {
		return store.Job{}, err
	}
	job, err := s.jobSvc.Enqueue(ctx, actorID, store.JobTypeDeprovisionSite, map[string]string{
		"site_id": site.ID,
	})
	if err != nil {
		_ = s.repo.UpdateSiteStatus(ctx, site.ID, store.SiteStatusFailed)
		return store.Job{}, err
	}
	return job, nil
}

func (s *Service) GetSiteContent(ctx context.Context, id, file string) (store.Site, string, string, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Site{}, "", "", err
	}

	name, err := resolveEditableFile(site, file, true)
	if err != nil {
		return store.Site{}, "", "", err
	}
	fullPath := filepath.Join(site.RootPath, name)
	b, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.Site{}, "", "", store.ErrNotFound
		}
		return store.Site{}, "", "", fmt.Errorf("read file: %w", err)
	}
	return site, name, string(b), nil
}

func (s *Service) UpdateSiteContent(ctx context.Context, id, file, content string) (store.Site, string, int, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Site{}, "", 0, err
	}
	if len(content) > maxSiteContentBytes {
		return store.Site{}, "", 0, ErrContentTooLong
	}

	name, err := resolveEditableFile(site, file, false)
	if err != nil {
		return store.Site{}, "", 0, err
	}
	if err := os.MkdirAll(site.RootPath, 0o755); err != nil {
		return store.Site{}, "", 0, fmt.Errorf("ensure site root: %w", err)
	}

	fullPath := filepath.Join(site.RootPath, name)
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return store.Site{}, "", 0, fmt.Errorf("write file: %w", err)
	}
	return site, name, len(content), nil
}

func normalizeDomain(in string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(in, ".")))
}

func isValidDomain(domain string) bool {
	if len(domain) < 3 || len(domain) > 253 {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return false
	}
	if !domainRegex.MatchString(domain) {
		return false
	}
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.HasPrefix(p, "-") || strings.HasSuffix(p, "-") {
			return false
		}
	}
	return true
}

func isValidRootPath(rootPath string) bool {
	if rootPath == "" || !strings.HasPrefix(rootPath, "/") {
		return false
	}
	clean := path.Clean(rootPath)
	if clean == "/" {
		return false
	}
	return !strings.Contains(clean, "..")
}

func resolveEditableFile(site store.Site, file string, preferExisting bool) (string, error) {
	name := strings.TrimSpace(strings.ToLower(file))
	if name != "" {
		if _, ok := editableFiles[name]; !ok {
			return "", ErrInvalidFile
		}
		return name, nil
	}

	if preferExisting {
		for _, candidate := range fileCandidatesByRuntime(site.Runtime) {
			fullPath := filepath.Join(site.RootPath, candidate)
			if _, err := os.Stat(fullPath); err == nil {
				return candidate, nil
			}
		}
	}
	return defaultEditableFile(site.Runtime), nil
}

func defaultEditableFile(runtimeName string) string {
	switch strings.ToLower(strings.TrimSpace(runtimeName)) {
	case "php":
		return "index.php"
	default:
		return "index.html"
	}
}

func fileCandidatesByRuntime(runtimeName string) []string {
	if strings.ToLower(strings.TrimSpace(runtimeName)) == "php" {
		return []string{"index.php", "index.html", "index.htm"}
	}
	return []string{"index.html", "index.htm", "index.php"}
}
