package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nusantara/internal/audit"
	backupsvc "nusantara/internal/backup"
	"nusantara/internal/config"
	dbsvc "nusantara/internal/db"
	"nusantara/internal/httpserver"
	"nusantara/internal/jobs"
	"nusantara/internal/monitor"
	"nusantara/internal/platform/oscheck"
	"nusantara/internal/provision"
	authsvc "nusantara/internal/service/auth"
	sitessvc "nusantara/internal/service/sites"
	sslsvc "nusantara/internal/ssl"
	"nusantara/internal/store/filedb"
	"nusantara/internal/updater"
)

type App struct {
	cfg    config.Config
	logger *log.Logger
}

func New(cfg config.Config, logger *log.Logger) *App {
	return &App{
		cfg:    cfg,
		logger: logger,
	}
}

func (a *App) Run() error {
	check := oscheck.Detect()
	if !check.Supported && !a.cfg.AllowNonUbuntu {
		return fmt.Errorf("unsupported host OS (%s %s), set NUSANTARA_ALLOW_NON_UBUNTU=true for non-production runs", check.ID, check.VersionID)
	}

	a.logger.Printf("host=%s %s supported=%t", check.ID, check.VersionID, check.Supported)

	if err := os.MkdirAll(a.cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	repo, err := filedb.New(a.cfg.DBPath)
	if err != nil {
		return fmt.Errorf("init repository: %w", err)
	}
	defer func() {
		_ = repo.Close()
	}()

	if err := repo.Migrate(context.Background()); err != nil {
		return fmt.Errorf("migrate repository: %w", err)
	}

	authService := authsvc.NewService(repo, time.Duration(a.cfg.TokenTTLHours)*time.Hour)
	if err := authService.EnsureBootstrapAdmin(
		context.Background(),
		a.cfg.BootstrapAdminUsername,
		a.cfg.BootstrapAdminPassword,
	); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	a.logger.Printf("bootstrap admin ensured username=%s", a.cfg.BootstrapAdminUsername)

	siteProvisioner := provision.NewNginxProvisioner(provision.NginxConfig{
		Apply:         a.cfg.ProvisionApply,
		AvailableDir:  a.cfg.NginxAvailableDir,
		EnabledDir:    a.cfg.NginxEnabledDir,
		TestCommand:   a.cfg.NginxTestCommand,
		ReloadCommand: a.cfg.NginxReloadCommand,
	}, a.logger)
	a.logger.Printf(
		"provision apply=%t available_dir=%s enabled_dir=%s",
		a.cfg.ProvisionApply,
		a.cfg.NginxAvailableDir,
		a.cfg.NginxEnabledDir,
	)

	jobService := jobs.NewService(repo, a.logger, siteProvisioner)
	jobService.Start(context.Background())
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = jobService.Stop(stopCtx)
	}()

	siteService := sitessvc.NewService(repo, jobService)
	auditService := audit.NewService(repo, a.logger)
	servicesMonitor := monitor.NewServicesMonitor(nil, 3*time.Second)
	sslService := sslsvc.NewService(a.cfg.ProvisionApply, a.cfg.CertbotCommand, 2*time.Minute, a.logger)
	dbService := dbsvc.NewService(a.cfg.ProvisionApply, a.cfg.MySQLCommand, 10*time.Second, a.logger)
	backupService := backupsvc.NewService(a.cfg.ProvisionApply, a.cfg.DBPath, a.cfg.BackupDir, a.logger)
	updaterService := updater.NewService(a.cfg.ProvisionApply, updater.Config{
		RepoURL:   a.cfg.UpdateRepoURL,
		Branch:    a.cfg.UpdateBranch,
		ScriptURL: a.cfg.UpdateScriptURL,
		UnitName:  a.cfg.UpdateUnitName,
		LogLines:  a.cfg.UpdateLogLines,
	}, a.logger)
	api := httpserver.NewAPI(authService, siteService, jobService, auditService, dbService, backupService, sslService, servicesMonitor, updaterService)

	server := &http.Server{
		Addr:         a.cfg.Address,
		Handler:      httpserver.NewRouter(check, api),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Printf("nusantarad listening on %s", a.cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.ShutdownSecs)*time.Second)
	defer cancel()

	a.logger.Printf("shutting down")
	return server.Shutdown(shutdownCtx)
}
