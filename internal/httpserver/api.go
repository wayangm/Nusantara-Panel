package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nusantara/internal/audit"
	backupsvc "nusantara/internal/backup"
	"nusantara/internal/buildinfo"
	dbsvc "nusantara/internal/db"
	"nusantara/internal/jobs"
	"nusantara/internal/monitor"
	"nusantara/internal/security/ratelimit"
	authsvc "nusantara/internal/service/auth"
	sitessvc "nusantara/internal/service/sites"
	sslsvc "nusantara/internal/ssl"
	"nusantara/internal/store"
	"nusantara/internal/updater"
)

type API struct {
	auth            *authsvc.Service
	sites           *sitessvc.Service
	jobs            *jobs.Service
	audit           *audit.Service
	db              *dbsvc.Service
	backup          *backupsvc.Service
	ssl             *sslsvc.Service
	servicesMonitor *monitor.ServicesMonitor
	loginLimiter    *ratelimit.LoginLimiter
	updater         *updater.Service
}

type principalContextKey struct{}
type tokenContextKey struct{}

func NewAPI(
	auth *authsvc.Service,
	sites *sitessvc.Service,
	jobs *jobs.Service,
	audit *audit.Service,
	db *dbsvc.Service,
	backup *backupsvc.Service,
	ssl *sslsvc.Service,
	servicesMonitor *monitor.ServicesMonitor,
	updaterSvc *updater.Service,
) *API {
	return &API{
		auth:            auth,
		sites:           sites,
		jobs:            jobs,
		audit:           audit,
		db:              db,
		backup:          backup,
		ssl:             ssl,
		servicesMonitor: servicesMonitor,
		loginLimiter:    ratelimit.NewLoginLimiter(5, 5*time.Minute),
		updater:         updaterSvc,
	}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/login", a.handleLogin)
	mux.Handle("POST /v1/auth/logout", a.requireAuth(http.HandlerFunc(a.handleLogout)))
	mux.Handle("GET /v1/auth/me", a.requireAuth(http.HandlerFunc(a.handleMe)))
	mux.Handle("POST /v1/auth/change-password", a.requireAuth(http.HandlerFunc(a.handleChangePassword)))

	mux.Handle("GET /v1/sites", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleListSites)))
	mux.Handle("POST /v1/sites", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleCreateSite)))
	mux.Handle("GET /v1/sites/{siteID}", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleGetSite)))
	mux.Handle("GET /v1/sites/{siteID}/content", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleGetSiteContent)))
	mux.Handle("PUT /v1/sites/{siteID}/content", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleUpdateSiteContent)))
	mux.Handle("GET /v1/sites/{siteID}/files", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleListSiteFiles)))
	mux.Handle("GET /v1/sites/{siteID}/files/download", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleDownloadSiteFile)))
	mux.Handle("POST /v1/sites/{siteID}/files/upload", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleUploadSiteFile)))
	mux.Handle("DELETE /v1/sites/{siteID}/files", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleDeleteSiteFile)))
	mux.Handle("POST /v1/sites/{siteID}/dirs", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleCreateSiteDir)))
	mux.Handle("DELETE /v1/sites/{siteID}/dirs", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleDeleteSiteDir)))
	mux.Handle("POST /v1/sites/{siteID}/backup", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleBackupSiteContent)))
	mux.Handle("DELETE /v1/sites/{siteID}", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleDeleteSite)))

	mux.Handle("GET /v1/jobs", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleListJobs)))
	mux.Handle("GET /v1/jobs/{jobID}", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleGetJob)))
	mux.Handle("GET /v1/db/databases", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleListDatabases)))
	mux.Handle("POST /v1/db/databases", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleCreateDatabase)))
	mux.Handle("POST /v1/db/users", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleCreateDatabaseUser)))
	mux.Handle("POST /v1/backup/run", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleRunBackup)))
	mux.Handle("POST /v1/backup/restore", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleRestoreBackup)))
	mux.Handle("POST /v1/ssl/issue", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleIssueSSL)))
	mux.Handle("POST /v1/ssl/renew", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleRenewSSL)))

	mux.Handle("GET /v1/audit/logs", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleListAuditLogs)))

	mux.Handle("GET /v1/monitor/host", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleMonitorHost)))
	mux.Handle("GET /v1/monitor/services", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleMonitorServices)))
	mux.Handle("GET /v1/panel/version", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handlePanelVersion)))
	mux.Handle("GET /v1/panel/update/check", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handlePanelUpdateCheck)))
	mux.Handle("POST /v1/panel/update", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handleStartPanelUpdate)))
	mux.Handle("GET /v1/panel/update/status", a.requireRole(store.RoleAdmin, http.HandlerFunc(a.handlePanelUpdateStatus)))
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := loginLimitKey(r, req.Username)
	if !a.loginLimiter.Allow(key, time.Now().UTC()) {
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}

	token, expiresAt, user, err := a.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, authsvc.ErrInvalidCredentials) {
			a.loginLimiter.RegisterFailure(key, time.Now().UTC())
			a.audit.Record(r.Context(), "", "auth.login.failed", "user", strings.TrimSpace(req.Username), nil)
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	a.loginLimiter.RegisterSuccess(key)

	a.audit.Record(r.Context(), user.ID, "auth.login.success", "user", user.ID, map[string]any{
		"username": user.Username,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": expiresAt,
		"user": map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *API) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.auth.ChangePassword(r.Context(), user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidPassword):
			writeError(w, http.StatusUnauthorized, "invalid current password")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	a.audit.Record(r.Context(), user.ID, "auth.change_password", "user", user.ID, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	token, _ := r.Context().Value(tokenContextKey{}).(string)
	user, _ := r.Context().Value(principalContextKey{}).(store.User)
	if err := a.auth.Logout(r.Context(), token); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	a.audit.Record(r.Context(), user.ID, "auth.logout", "user", user.ID, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

type createSiteRequest struct {
	Domain   string `json:"domain"`
	RootPath string `json:"root_path"`
	Runtime  string `json:"runtime"`
}

func (a *API) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createSiteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	site, job, err := a.sites.CreateSite(r.Context(), user.ID, sitessvc.CreateSiteInput{
		Domain:   req.Domain,
		RootPath: req.RootPath,
		Runtime:  req.Runtime,
	})
	if err != nil {
		switch {
		case errors.Is(err, sitessvc.ErrInvalidDomain), errors.Is(err, sitessvc.ErrInvalidRoot), errors.Is(err, sitessvc.ErrInvalidRuntime):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "domain already exists")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "site.create", "site", site.ID, map[string]any{
		"domain": site.Domain,
		"job_id": job.ID,
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"site": site,
		"job":  job,
	})
}

func (a *API) handleListSites(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 200)
	sites, err := a.sites.ListSites(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sites})
}

func (a *API) handleGetSite(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	site, err := a.sites.GetSite(r.Context(), siteID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "site not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, site)
}

type updateSiteContentRequest struct {
	File    string `json:"file"`
	Content string `json:"content"`
}

type uploadSiteFileRequest struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
}

type siteDirRequest struct {
	Path string `json:"path"`
}

func (a *API) handleGetSiteContent(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	file := r.URL.Query().Get("file")
	site, name, content, err := a.sites.GetSiteContent(r.Context(), siteID, file)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site or file not found")
		case errors.Is(err, sitessvc.ErrInvalidFile):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_id":   site.ID,
		"domain":    site.Domain,
		"root_path": site.RootPath,
		"file":      name,
		"size":      len(content),
		"content":   content,
	})
}

func (a *API) handleUpdateSiteContent(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	siteID := r.PathValue("siteID")
	var req updateSiteContentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	site, name, size, err := a.sites.UpdateSiteContent(r.Context(), siteID, req.File, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site not found")
		case errors.Is(err, sitessvc.ErrInvalidFile), errors.Is(err, sitessvc.ErrContentTooLong):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "site.content.update", "site", site.ID, map[string]any{
		"file": name,
		"size": size,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"site_id":   site.ID,
		"domain":    site.Domain,
		"root_path": site.RootPath,
		"file":      name,
		"size":      size,
	})
}

func (a *API) handleListSiteFiles(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	dir := r.URL.Query().Get("dir")
	limit := parseLimit(r, 200)

	site, relDir, items, err := a.sites.ListSiteFiles(r.Context(), siteID, dir, limit)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site or directory not found")
		case errors.Is(err, sitessvc.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"site_id":   site.ID,
		"domain":    site.Domain,
		"root_path": site.RootPath,
		"dir":       relDir,
		"items":     items,
	})
}

func (a *API) handleDownloadSiteFile(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	relPath := r.URL.Query().Get("path")
	_, cleanPath, content, err := a.sites.DownloadSiteFile(r.Context(), siteID, relPath)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site or file not found")
		case errors.Is(err, sitessvc.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	name := path.Base(cleanPath)
	contentType := mime.TypeByExtension(path.Ext(cleanPath))
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (a *API) handleUploadSiteFile(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	siteID := r.PathValue("siteID")
	var req uploadSiteFileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	site, relPath, size, err := a.sites.UploadSiteFile(r.Context(), siteID, req.Path, req.ContentBase64)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site not found")
		case errors.Is(err, sitessvc.ErrInvalidPath), errors.Is(err, sitessvc.ErrInvalidBase64), errors.Is(err, sitessvc.ErrFileTooLarge):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "site.file.upload", "site", site.ID, map[string]any{
		"path": relPath,
		"size": size,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"site_id":   site.ID,
		"domain":    site.Domain,
		"root_path": site.RootPath,
		"path":      relPath,
		"size":      size,
	})
}

func (a *API) handleDeleteSiteFile(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	siteID := r.PathValue("siteID")
	relPath := r.URL.Query().Get("path")
	site, deletedPath, err := a.sites.DeleteSiteFile(r.Context(), siteID, relPath)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site or file not found")
		case errors.Is(err, sitessvc.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "site.file.delete", "site", site.ID, map[string]any{
		"path": deletedPath,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"site_id": site.ID,
		"domain":  site.Domain,
		"path":    deletedPath,
	})
}

func (a *API) handleCreateSiteDir(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	siteID := r.PathValue("siteID")
	var req siteDirRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	site, relPath, err := a.sites.CreateSiteDirectory(r.Context(), siteID, req.Path)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site not found")
		case errors.Is(err, sitessvc.ErrInvalidPath):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "site.dir.create", "site", site.ID, map[string]any{
		"path": relPath,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":  "ok",
		"site_id": site.ID,
		"domain":  site.Domain,
		"path":    relPath,
	})
}

func (a *API) handleDeleteSiteDir(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	siteID := r.PathValue("siteID")
	relPath := r.URL.Query().Get("path")
	recursive := false
	if raw := strings.TrimSpace(r.URL.Query().Get("recursive")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid recursive flag")
			return
		}
		recursive = parsed
	}

	site, deletedPath, err := a.sites.DeleteSiteDirectory(r.Context(), siteID, relPath, recursive)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site or directory not found")
		case errors.Is(err, sitessvc.ErrInvalidPath), errors.Is(err, sitessvc.ErrNotDirectory):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, sitessvc.ErrDirNotEmpty):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "site.dir.delete", "site", site.ID, map[string]any{
		"path":      deletedPath,
		"recursive": recursive,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"site_id":   site.ID,
		"domain":    site.Domain,
		"path":      deletedPath,
		"recursive": recursive,
	})
}

func (a *API) handleBackupSiteContent(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	siteID := r.PathValue("siteID")
	site, result, err := a.sites.BackupSiteContent(r.Context(), siteID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "site not found")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "site.backup.content", "site", site.ID, map[string]any{
		"file": result.File,
		"size": result.Size,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "ok",
		"site_id":    site.ID,
		"domain":     site.Domain,
		"file":       result.File,
		"size":       result.Size,
		"created_at": result.CreatedAt,
	})
}

func (a *API) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	siteID := r.PathValue("siteID")
	job, err := a.sites.DeleteSite(r.Context(), user.ID, siteID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "site not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	a.audit.Record(r.Context(), user.ID, "site.delete.requested", "site", siteID, map[string]any{
		"job_id": job.ID,
	})
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "deprovisioning",
		"job":    job,
	})
}

func (a *API) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 200)
	items, err := a.jobs.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID")
	job, err := a.jobs.Get(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

type createDatabaseRequest struct {
	Name string `json:"name"`
}

func (a *API) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeError(w, http.StatusInternalServerError, "db service is not configured")
		return
	}
	items, err := a.db.ListDatabases(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if a.db == nil {
		writeError(w, http.StatusInternalServerError, "db service is not configured")
		return
	}

	var req createDatabaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := a.db.CreateDatabase(r.Context(), req.Name); err != nil {
		if errors.Is(err, dbsvc.ErrInvalidDatabaseName) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit.Record(r.Context(), user.ID, "db.create_database", "database", req.Name, nil)
	writeJSON(w, http.StatusCreated, map[string]string{
		"status":   "ok",
		"database": req.Name,
	})
}

type createDatabaseUserRequest struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

func (a *API) handleCreateDatabaseUser(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if a.db == nil {
		writeError(w, http.StatusInternalServerError, "db service is not configured")
		return
	}

	var req createDatabaseUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := a.db.CreateUser(r.Context(), dbsvc.CreateUserInput{
		Database: req.Database,
		Username: req.Username,
		Password: req.Password,
		Host:     req.Host,
	}); err != nil {
		switch {
		case errors.Is(err, dbsvc.ErrInvalidDatabaseName),
			errors.Is(err, dbsvc.ErrInvalidUsername),
			errors.Is(err, dbsvc.ErrInvalidPassword),
			errors.Is(err, dbsvc.ErrInvalidHost):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	host := req.Host
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	a.audit.Record(r.Context(), user.ID, "db.create_user", "database", req.Database, map[string]any{
		"username": req.Username,
		"host":     host,
	})
	writeJSON(w, http.StatusCreated, map[string]string{
		"status":   "ok",
		"database": req.Database,
		"username": req.Username,
		"host":     host,
	})
}

func (a *API) handleRunBackup(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if a.backup == nil {
		writeError(w, http.StatusInternalServerError, "backup service is not configured")
		return
	}

	result, err := a.backup.Run(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit.Record(r.Context(), user.ID, "backup.run", "file", result.File, nil)
	writeJSON(w, http.StatusCreated, result)
}

type restoreBackupRequest struct {
	File string `json:"file"`
}

func (a *API) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if a.backup == nil {
		writeError(w, http.StatusInternalServerError, "backup service is not configured")
		return
	}

	var req restoreBackupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.backup.Restore(r.Context(), req.File); err != nil {
		if errors.Is(err, backupsvc.ErrInvalidBackupPath) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit.Record(r.Context(), user.ID, "backup.restore", "file", req.File, nil)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"file":   req.File,
		"note":   "restart nusantara-panel service is recommended after restore",
	})
}

func (a *API) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 200)
	items, err := a.audit.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type issueSSLRequest struct {
	Domain string `json:"domain"`
	Email  string `json:"email"`
}

func (a *API) handleIssueSSL(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if a.ssl == nil {
		writeError(w, http.StatusInternalServerError, "ssl service is not configured")
		return
	}

	var req issueSSLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.ssl.Issue(r.Context(), req.Domain, req.Email); err != nil {
		switch {
		case errors.Is(err, sslsvc.ErrInvalidDomain), errors.Is(err, sslsvc.ErrInvalidEmail):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	a.audit.Record(r.Context(), user.ID, "ssl.issue", "domain", req.Domain, map[string]any{
		"email": req.Email,
	})
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"domain": req.Domain,
	})
}

func (a *API) handleRenewSSL(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if a.ssl == nil {
		writeError(w, http.StatusInternalServerError, "ssl service is not configured")
		return
	}
	if err := a.ssl.Renew(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit.Record(r.Context(), user.ID, "ssl.renew", "system", "certbot", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleMonitorHost(w http.ResponseWriter, _ *http.Request) {
	hostname, _ := os.Hostname()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"hostname":    hostname,
		"goos":        runtime.GOOS,
		"goarch":      runtime.GOARCH,
		"cpu_threads": runtime.NumCPU(),
		"timestamp":   time.Now().UTC(),
	})
}

func (a *API) handleMonitorServices(w http.ResponseWriter, r *http.Request) {
	if a.servicesMonitor == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"items": []map[string]any{},
			"note":  "services monitor is not configured",
		})
		return
	}
	items, err := a.servicesMonitor.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read service status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (a *API) handlePanelVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    buildinfo.Version,
		"commit":     buildinfo.Commit,
		"build_time": buildinfo.BuildTime,
		"go_version": runtime.Version(),
		"timestamp":  time.Now().UTC(),
	})
}

func (a *API) handleStartPanelUpdate(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(principalContextKey{}).(store.User)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if a.updater == nil {
		writeError(w, http.StatusInternalServerError, "panel updater is not configured")
		return
	}

	result, err := a.updater.Start(r.Context())
	if err != nil {
		statusPayload := map[string]any{}
		if st, stErr := a.updater.Status(r.Context()); stErr == nil {
			statusPayload["update_status"] = st
		}
		switch {
		case errors.Is(err, updater.ErrAlreadyRunning):
			writeJSON(w, http.StatusConflict, mergeErrorPayload("panel update is already running", statusPayload))
		case errors.Is(err, updater.ErrCooldown):
			writeJSON(w, http.StatusTooManyRequests, mergeErrorPayload(err.Error(), statusPayload))
		case errors.Is(err, updater.ErrDisabled):
			writeJSON(w, http.StatusBadRequest, mergeErrorPayload("panel updater is disabled", statusPayload))
		default:
			writeJSON(w, http.StatusInternalServerError, mergeErrorPayload(err.Error(), statusPayload))
		}
		return
	}
	a.audit.Record(r.Context(), user.ID, "panel.update.start", "system", result.Unit, map[string]any{
		"repo_url":   result.RepoURL,
		"branch":     result.Branch,
		"script_url": result.ScriptURL,
	})
	writeJSON(w, http.StatusAccepted, result)
}

func (a *API) handlePanelUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if a.updater == nil {
		writeError(w, http.StatusInternalServerError, "panel updater is not configured")
		return
	}
	result, err := a.updater.Check(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, updater.ErrDisabled):
			writeError(w, http.StatusBadRequest, "panel updater is disabled")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) handlePanelUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if a.updater == nil {
		writeError(w, http.StatusInternalServerError, "panel updater is not configured")
		return
	}
	status, err := a.updater.Status(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, updater.ErrDisabled):
			writeError(w, http.StatusBadRequest, "panel updater is disabled")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func mergeErrorPayload(message string, extra map[string]any) map[string]any {
	payload := map[string]any{
		"error":     message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range extra {
		payload[k] = v
	}
	return payload
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, err := a.auth.Authenticate(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, user)
		ctx = context.WithValue(ctx, tokenContextKey{}, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) requireRole(role string, next http.Handler) http.Handler {
	return a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(principalContextKey{}).(store.User)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if user.Role != role {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func bearerToken(value string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(value), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("missing bearer token")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty token")
	}
	return token, nil
}

func decodeJSON(r *http.Request, out any) error {
	reader := io.LimitReader(r.Body, 1<<20)
	defer r.Body.Close()
	dec := json.NewDecoder(reader)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid json body")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("invalid json body")
	}
	return nil
}

func parseLimit(r *http.Request, fallback int) int {
	if fallback <= 0 {
		fallback = 100
	}
	v := strings.TrimSpace(r.URL.Query().Get("limit"))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	if n > 500 {
		return 500
	}
	return n
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error":     message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func loginLimitKey(r *http.Request, username string) string {
	ip := strings.TrimSpace(r.RemoteAddr)
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		ip = strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	} else if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	username = strings.ToLower(strings.TrimSpace(username))
	return ip + ":" + username
}
