package sites

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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
	ErrNotDirectory   = errors.New("path is not a directory")
	ErrDirNotEmpty    = errors.New("directory is not empty")
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
	repo      store.Repository
	jobSvc    *jobs.Service
	backupDir string
	apply     bool
}

type CreateSiteInput struct {
	Domain   string
	RootPath string
	Runtime  string
}

func NewService(repo store.Repository, jobSvc *jobs.Service, backupDir string, apply bool) *Service {
	return &Service{
		repo:      repo,
		jobSvc:    jobSvc,
		backupDir: backupDir,
		apply:     apply,
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

func (s *Service) DownloadSiteFile(ctx context.Context, id, relPath string) (store.Site, string, []byte, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Site{}, "", nil, err
	}

	cleanPath, err := normalizeRelativePath(relPath, false)
	if err != nil {
		return store.Site{}, "", nil, err
	}
	fullPath := filepath.Join(site.RootPath, filepath.FromSlash(cleanPath))
	if !isWithinRoot(site.RootPath, fullPath) {
		return store.Site{}, "", nil, ErrInvalidPath
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.Site{}, "", nil, store.ErrNotFound
		}
		return store.Site{}, "", nil, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return store.Site{}, "", nil, ErrInvalidPath
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return store.Site{}, "", nil, fmt.Errorf("read file: %w", err)
	}
	return site, cleanPath, content, nil
}

func (s *Service) CreateSiteDirectory(ctx context.Context, id, relPath string) (store.Site, string, error) {
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

	if err := os.MkdirAll(fullPath, 0o755); err != nil {
		return store.Site{}, "", fmt.Errorf("create directory: %w", err)
	}
	return site, cleanPath, nil
}

func (s *Service) DeleteSiteDirectory(ctx context.Context, id, relPath string, recursive bool) (store.Site, string, error) {
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
		return store.Site{}, "", fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return store.Site{}, "", ErrNotDirectory
	}

	if recursive {
		if err := os.RemoveAll(fullPath); err != nil {
			return store.Site{}, "", fmt.Errorf("delete directory: %w", err)
		}
		return site, cleanPath, nil
	}

	if err := os.Remove(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.Site{}, "", store.ErrNotFound
		}
		if errors.Is(err, os.ErrPermission) {
			return store.Site{}, "", fmt.Errorf("delete directory: %w", err)
		}
		return store.Site{}, "", ErrDirNotEmpty
	}
	return site, cleanPath, nil
}

type SiteContentBackupResult struct {
	File      string    `json:"file"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

func (s *Service) BackupSiteContent(ctx context.Context, id string) (store.Site, SiteContentBackupResult, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if err != nil {
		return store.Site{}, SiteContentBackupResult{}, err
	}
	if strings.TrimSpace(s.backupDir) == "" {
		return store.Site{}, SiteContentBackupResult{}, errors.New("backup dir is empty")
	}

	now := time.Now().UTC()
	fileName := fmt.Sprintf("site_content_%s_%s.zip", sanitizeDomainForFile(site.Domain), now.Format("20060102_150405"))
	targetDir := filepath.Join(s.backupDir, "sites", sanitizeDomainForFile(site.Domain))
	target := filepath.Join(targetDir, fileName)

	if !s.apply {
		return site, SiteContentBackupResult{
			File:      target,
			CreatedAt: now,
			Size:      0,
		}, nil
	}

	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return store.Site{}, SiteContentBackupResult{}, fmt.Errorf("create backup dir: %w", err)
	}

	tmp := target + ".tmp"
	if err := zipDirectory(site.RootPath, tmp); err != nil {
		_ = os.Remove(tmp)
		return store.Site{}, SiteContentBackupResult{}, err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return store.Site{}, SiteContentBackupResult{}, fmt.Errorf("move backup file: %w", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		return store.Site{}, SiteContentBackupResult{}, fmt.Errorf("stat backup file: %w", err)
	}

	return site, SiteContentBackupResult{
		File:      target,
		CreatedAt: now,
		Size:      info.Size(),
	}, nil
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

func sanitizeDomainForFile(domain string) string {
	clean := strings.ToLower(strings.TrimSpace(domain))
	clean = strings.ReplaceAll(clean, "..", ".")
	clean = strings.ReplaceAll(clean, "/", "-")
	clean = strings.ReplaceAll(clean, "\\", "-")
	if clean == "" {
		return "site"
	}
	return clean
}

func zipDirectory(rootPath, targetZip string) error {
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.ErrNotFound
		}
		return fmt.Errorf("stat site root: %w", err)
	}
	if !rootInfo.IsDir() {
		return ErrNotDirectory
	}

	out, err := os.OpenFile(targetZip, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("create zip file: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)

	err = filepath.Walk(rootPath, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == rootPath {
			return nil
		}

		rel, relErr := filepath.Rel(rootPath, current)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if info.IsDir() {
			_, dirErr := zw.Create(rel + "/")
			return dirErr
		}

		header, hErr := zip.FileInfoHeader(info)
		if hErr != nil {
			return hErr
		}
		header.Name = rel
		header.Method = zip.Deflate
		writer, cErr := zw.CreateHeader(header)
		if cErr != nil {
			return cErr
		}

		f, oErr := os.Open(current)
		if oErr != nil {
			return oErr
		}
		_, cpErr := io.Copy(writer, f)
		closeErr := f.Close()
		if cpErr != nil {
			return cpErr
		}
		return closeErr
	})
	if err != nil {
		return fmt.Errorf("zip site content: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}
	return nil
}
