package sites

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
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
	ErrInvalidPath    = errors.New("invalid path")
	ErrInvalidBase64  = errors.New("invalid base64 content")
	ErrFileTooLarge   = errors.New("file too large")
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
const maxSiteUploadBytes = 8 * 1024 * 1024

type SiteFileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Type    string    `json:"type"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

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

func (s *Service) ListSiteFiles(ctx context.Context, id, dir string, limit int) (store.Site, string, []SiteFileEntry, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Site{}, "", nil, err
	}

	relDir, err := normalizeRelativePath(dir, true)
	if err != nil {
		return store.Site{}, "", nil, err
	}
	targetDir := filepath.Join(site.RootPath, filepath.FromSlash(relDir))
	if !isWithinRoot(site.RootPath, targetDir) {
		return store.Site{}, "", nil, ErrInvalidPath
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.Site{}, "", nil, store.ErrNotFound
		}
		return store.Site{}, "", nil, fmt.Errorf("read directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}

	items := make([]SiteFileEntry, 0, len(entries))
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		relPath := entry.Name()
		if relDir != "" {
			relPath = path.Join(relDir, entry.Name())
		}
		itemType := "file"
		size := info.Size()
		if entry.IsDir() {
			itemType = "dir"
			size = 0
		}
		items = append(items, SiteFileEntry{
			Name:    entry.Name(),
			Path:    relPath,
			Type:    itemType,
			Size:    size,
			ModTime: info.ModTime().UTC(),
		})
	}
	return site, relDir, items, nil
}

func (s *Service) UploadSiteFile(ctx context.Context, id, relPath, contentBase64 string) (store.Site, string, int, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Site{}, "", 0, err
	}

	cleanPath, err := normalizeRelativePath(relPath, false)
	if err != nil {
		return store.Site{}, "", 0, err
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(contentBase64))
	if err != nil {
		return store.Site{}, "", 0, ErrInvalidBase64
	}
	if len(data) > maxSiteUploadBytes {
		return store.Site{}, "", 0, ErrFileTooLarge
	}

	fullPath := filepath.Join(site.RootPath, filepath.FromSlash(cleanPath))
	if !isWithinRoot(site.RootPath, fullPath) {
		return store.Site{}, "", 0, ErrInvalidPath
	}
	if info, statErr := os.Stat(fullPath); statErr == nil && info.IsDir() {
		return store.Site{}, "", 0, ErrInvalidPath
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return store.Site{}, "", 0, fmt.Errorf("ensure parent dir: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return store.Site{}, "", 0, fmt.Errorf("write file: %w", err)
	}
	return site, cleanPath, len(data), nil
}

func (s *Service) DeleteSiteFile(ctx context.Context, id, relPath string) (store.Site, string, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Site{}, "", err
	}

	cleanPath, err := normalizeRelativePath(relPath, false)
	if err != nil {
		return store.Site{}, "", err
	}
	fullPath := filepath.Join(site.RootPath, filepath.FromSlash(cleanPath))
	if !isWithinRoot(site.RootPath, fullPath) {
		return store.Site{}, "", ErrInvalidPath
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.Site{}, "", store.ErrNotFound
		}
		return store.Site{}, "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return store.Site{}, "", ErrInvalidPath
	}

	if err := os.Remove(fullPath); err != nil {
		return store.Site{}, "", fmt.Errorf("delete file: %w", err)
	}
	return site, cleanPath, nil
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

func normalizeRelativePath(raw string, allowEmpty bool) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if trimmed == "" {
		if allowEmpty {
			return "", nil
		}
		return "", ErrInvalidPath
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", ErrInvalidPath
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return "", ErrInvalidPath
		}
	}
	clean := path.Clean("/" + trimmed)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || clean == "" {
		if allowEmpty {
			return "", nil
		}
		return "", ErrInvalidPath
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", ErrInvalidPath
	}
	return clean, nil
}

func isWithinRoot(rootPath, targetPath string) bool {
	root := filepath.Clean(rootPath)
	target := filepath.Clean(targetPath)
	if target == root {
		return true
	}
	return strings.HasPrefix(target, root+string(os.PathSeparator))
}
