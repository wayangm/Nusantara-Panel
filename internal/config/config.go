package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

const (
	defaultAddress                = ":8080"
	defaultDataDir                = "/var/lib/nusantara-panel"
	defaultNginxAvailableDir      = "/etc/nginx/sites-available"
	defaultNginxEnabledDir        = "/etc/nginx/sites-enabled"
	defaultNginxTestCommand       = "nginx -t"
	defaultNginxReloadCommand     = "systemctl reload nginx"
	defaultCertbotCommand         = "certbot"
	defaultMySQLCommand           = "mysql"
	defaultBackupDir              = "/var/backups/nusantara-panel"
	defaultLogLevel               = "info"
	defaultShutdownSecs           = 10
	defaultAllowNonLinux          = false
	defaultTokenTTLHours          = 24
	defaultBootstrapAdminUsername = "admin"
	defaultBootstrapAdminPassword = ""
	defaultUpdateRepoURL          = "https://github.com/wayangm/Nusantara-Panel.git"
	defaultUpdateBranch           = "main"
	defaultUpdateScriptURL        = "https://raw.githubusercontent.com/wayangm/Nusantara-Panel/main/install.sh"
	defaultUpdateUnitName         = "nusantara-panel-updater"
	defaultUpdateLogLines         = 80
	defaultUpdateCooldownSecs     = 20
)

type Config struct {
	Address            string
	DataDir            string
	DBPath             string
	ProvisionApply     bool
	NginxAvailableDir  string
	NginxEnabledDir    string
	NginxTestCommand   string
	NginxReloadCommand string
	CertbotCommand     string
	MySQLCommand       string
	BackupDir          string
	LogLevel           string
	ShutdownSecs       int
	TokenTTLHours      int
	AllowNonUbuntu     bool

	BootstrapAdminUsername string
	BootstrapAdminPassword string

	UpdateRepoURL   string
	UpdateBranch    string
	UpdateScriptURL string
	UpdateUnitName  string
	UpdateLogLines  int
	UpdateCooldown  int
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		Address:            getenv("NUSANTARA_ADDR", defaultAddress),
		DataDir:            getenv("NUSANTARA_DATA_DIR", defaultDataDir),
		ProvisionApply:     runtime.GOOS == "linux",
		NginxAvailableDir:  getenv("NUSANTARA_NGINX_SITES_AVAILABLE_DIR", defaultNginxAvailableDir),
		NginxEnabledDir:    getenv("NUSANTARA_NGINX_SITES_ENABLED_DIR", defaultNginxEnabledDir),
		NginxTestCommand:   getenv("NUSANTARA_NGINX_TEST_COMMAND", defaultNginxTestCommand),
		NginxReloadCommand: getenv("NUSANTARA_NGINX_RELOAD_COMMAND", defaultNginxReloadCommand),
		CertbotCommand:     getenv("NUSANTARA_CERTBOT_COMMAND", defaultCertbotCommand),
		MySQLCommand:       getenv("NUSANTARA_MYSQL_COMMAND", defaultMySQLCommand),
		BackupDir:          getenv("NUSANTARA_BACKUP_DIR", defaultBackupDir),
		LogLevel:           getenv("NUSANTARA_LOG_LEVEL", defaultLogLevel),
		ShutdownSecs:       defaultShutdownSecs,
		TokenTTLHours:      defaultTokenTTLHours,
		AllowNonUbuntu:     defaultAllowNonLinux,

		BootstrapAdminUsername: getenv("NUSANTARA_BOOTSTRAP_ADMIN_USERNAME", defaultBootstrapAdminUsername),
		BootstrapAdminPassword: getenv("NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD", defaultBootstrapAdminPassword),
		UpdateRepoURL:          getenv("NUSANTARA_UPDATE_REPO_URL", defaultUpdateRepoURL),
		UpdateBranch:           getenv("NUSANTARA_UPDATE_BRANCH", defaultUpdateBranch),
		UpdateScriptURL:        getenv("NUSANTARA_UPDATE_SCRIPT_URL", defaultUpdateScriptURL),
		UpdateUnitName:         getenv("NUSANTARA_UPDATE_UNIT_NAME", defaultUpdateUnitName),
		UpdateLogLines:         defaultUpdateLogLines,
		UpdateCooldown:         defaultUpdateCooldownSecs,
	}

	if v := os.Getenv("NUSANTARA_SHUTDOWN_SECS"); v != "" {
		secs, err := strconv.Atoi(v)
		if err != nil || secs < 1 {
			return Config{}, fmt.Errorf("invalid NUSANTARA_SHUTDOWN_SECS: %q", v)
		}
		cfg.ShutdownSecs = secs
	}

	if v := os.Getenv("NUSANTARA_ALLOW_NON_UBUNTU"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid NUSANTARA_ALLOW_NON_UBUNTU: %q", v)
		}
		cfg.AllowNonUbuntu = b
	}

	if v := os.Getenv("NUSANTARA_TOKEN_TTL_HOURS"); v != "" {
		ttl, err := strconv.Atoi(v)
		if err != nil || ttl < 1 {
			return Config{}, fmt.Errorf("invalid NUSANTARA_TOKEN_TTL_HOURS: %q", v)
		}
		cfg.TokenTTLHours = ttl
	}

	if v := os.Getenv("NUSANTARA_PROVISION_APPLY"); v != "" {
		apply, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid NUSANTARA_PROVISION_APPLY: %q", v)
		}
		cfg.ProvisionApply = apply
	}

	if v := os.Getenv("NUSANTARA_UPDATE_LOG_LINES"); v != "" {
		lines, err := strconv.Atoi(v)
		if err != nil || lines < 1 {
			return Config{}, fmt.Errorf("invalid NUSANTARA_UPDATE_LOG_LINES: %q", v)
		}
		cfg.UpdateLogLines = lines
	}

	if v := os.Getenv("NUSANTARA_UPDATE_COOLDOWN_SECS"); v != "" {
		secs, err := strconv.Atoi(v)
		if err != nil || secs < 0 {
			return Config{}, fmt.Errorf("invalid NUSANTARA_UPDATE_COOLDOWN_SECS: %q", v)
		}
		cfg.UpdateCooldown = secs
	}

	cfg.DBPath = getenv("NUSANTARA_DB_PATH", filepath.Join(cfg.DataDir, "nusantara_state.json"))

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
