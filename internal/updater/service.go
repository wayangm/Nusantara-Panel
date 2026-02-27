package updater

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrDisabled       = errors.New("panel updater is disabled")
	ErrAlreadyRunning = errors.New("panel updater is already running")
	ErrInvalidConfig  = errors.New("invalid panel updater config")
)

var (
	branchRe   = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	unitNameRe = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)
)

type Config struct {
	RepoURL   string
	Branch    string
	ScriptURL string
	UnitName  string
	LogLines  int
}

type Service struct {
	enabled bool
	cfg     Config
	logger  *log.Logger
}

type StartResult struct {
	Unit      string    `json:"unit"`
	RepoURL   string    `json:"repo_url"`
	Branch    string    `json:"branch"`
	ScriptURL string    `json:"script_url"`
	StartedAt time.Time `json:"started_at"`
}

type Status struct {
	Unit                 string    `json:"unit"`
	Exists               bool      `json:"exists"`
	Running              bool      `json:"running"`
	ActiveState          string    `json:"active_state"`
	SubState             string    `json:"sub_state"`
	Result               string    `json:"result"`
	ExecMainStatus       int       `json:"exec_main_status"`
	StateChangeTimestamp string    `json:"state_change_timestamp"`
	Success              bool      `json:"success"`
	Failed               bool      `json:"failed"`
	LastCheckedAt        time.Time `json:"last_checked_at"`
	Logs                 []string  `json:"logs,omitempty"`
}

func NewService(enabled bool, cfg Config, logger *log.Logger) *Service {
	if cfg.LogLines <= 0 {
		cfg.LogLines = 80
	}
	cfg.UnitName = normalizeUnitName(cfg.UnitName)
	return &Service{
		enabled: enabled,
		cfg:     cfg,
		logger:  logger,
	}
}

func (s *Service) Start(ctx context.Context) (StartResult, error) {
	if !s.enabled {
		return StartResult{}, ErrDisabled
	}
	if err := s.validateConfig(); err != nil {
		return StartResult{}, err
	}

	status, err := s.Status(ctx)
	if err == nil && status.Exists && status.Running {
		return StartResult{}, ErrAlreadyRunning
	}

	cmdScript := fmt.Sprintf(
		"set -euo pipefail; curl -fsSL %s -o /tmp/nusantara-install.sh; /bin/bash /tmp/nusantara-install.sh --repo %s --branch %s",
		shellQuote(s.cfg.ScriptURL),
		shellQuote(s.cfg.RepoURL),
		shellQuote(s.cfg.Branch),
	)

	cmd := exec.CommandContext(
		ctx,
		"systemd-run",
		"--unit",
		s.cfg.UnitName,
		"--property=Type=oneshot",
		"--property=RemainAfterExit=yes",
		"/bin/bash",
		"-lc",
		cmdScript,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		raw := strings.TrimSpace(string(output))
		if strings.Contains(strings.ToLower(raw), "already exists") {
			s.cleanupUnit(ctx)
			cmdRetry := exec.CommandContext(
				ctx,
				"systemd-run",
				"--unit",
				s.cfg.UnitName,
				"--property=Type=oneshot",
				"--property=RemainAfterExit=yes",
				"/bin/bash",
				"-lc",
				cmdScript,
			)
			retryOut, retryErr := cmdRetry.CombinedOutput()
			if retryErr != nil {
				return StartResult{}, fmt.Errorf("start updater unit: %w: %s", retryErr, strings.TrimSpace(string(retryOut)))
			}
		} else {
			return StartResult{}, fmt.Errorf("start updater unit: %w: %s", err, raw)
		}
	}

	s.logf("panel updater started unit=%s", s.unitServiceName())
	return StartResult{
		Unit:      s.unitServiceName(),
		RepoURL:   s.cfg.RepoURL,
		Branch:    s.cfg.Branch,
		ScriptURL: s.cfg.ScriptURL,
		StartedAt: time.Now().UTC(),
	}, nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	if !s.enabled {
		return Status{}, ErrDisabled
	}
	if err := s.validateConfig(); err != nil {
		return Status{}, err
	}

	props, err := s.showUnit(ctx)
	if err != nil {
		return Status{}, err
	}

	loadState := props["LoadState"]
	activeState := props["ActiveState"]
	subState := props["SubState"]
	result := props["Result"]
	execStatus := parseInt(props["ExecMainStatus"])
	running := activeState == "activating" || subState == "running" || subState == "start"
	exists := loadState != "not-found"
	success := exists && !running && result == "success" && execStatus == 0
	failed := exists && !running && (result == "failed" || execStatus != 0)

	status := Status{
		Unit:                 s.unitServiceName(),
		Exists:               exists,
		Running:              running,
		ActiveState:          activeState,
		SubState:             subState,
		Result:               result,
		ExecMainStatus:       execStatus,
		StateChangeTimestamp: props["StateChangeTimestamp"],
		Success:              success,
		Failed:               failed,
		LastCheckedAt:        time.Now().UTC(),
	}
	if exists {
		if logs, logErr := s.logs(ctx); logErr == nil {
			status.Logs = logs
		}
	}
	return status, nil
}

func (s *Service) validateConfig() error {
	if strings.TrimSpace(s.cfg.RepoURL) == "" || strings.TrimSpace(s.cfg.ScriptURL) == "" || strings.TrimSpace(s.cfg.Branch) == "" {
		return fmt.Errorf("%w: empty updater repo/script/branch", ErrInvalidConfig)
	}
	if _, err := url.ParseRequestURI(s.cfg.RepoURL); err != nil {
		return fmt.Errorf("%w: invalid repo url", ErrInvalidConfig)
	}
	if _, err := url.ParseRequestURI(s.cfg.ScriptURL); err != nil {
		return fmt.Errorf("%w: invalid script url", ErrInvalidConfig)
	}
	if !branchRe.MatchString(s.cfg.Branch) {
		return fmt.Errorf("%w: invalid branch", ErrInvalidConfig)
	}
	if !unitNameRe.MatchString(s.cfg.UnitName) {
		return fmt.Errorf("%w: invalid unit name", ErrInvalidConfig)
	}
	return nil
}

func (s *Service) showUnit(ctx context.Context) (map[string]string, error) {
	ctxShow, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctxShow,
		"systemctl",
		"show",
		s.unitServiceName(),
		"--property=LoadState",
		"--property=ActiveState",
		"--property=SubState",
		"--property=Result",
		"--property=ExecMainStatus",
		"--property=StateChangeTimestamp",
		"--no-pager",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("read updater unit status: %w: %s", err, strings.TrimSpace(string(output)))
	}
	result := map[string]string{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[parts[0]] = strings.TrimSpace(parts[1])
	}
	return result, nil
}

func (s *Service) logs(ctx context.Context) ([]string, error) {
	ctxLogs, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	lines := s.cfg.LogLines
	if lines < 20 {
		lines = 20
	}
	if lines > 500 {
		lines = 500
	}

	cmd := exec.CommandContext(
		ctxLogs,
		"journalctl",
		"-u",
		s.unitServiceName(),
		"-n",
		strconv.Itoa(lines),
		"--no-pager",
		"--output=short-iso",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("read updater logs: %w", err)
	}
	raw := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

func (s *Service) cleanupUnit(ctx context.Context) {
	ctxCleanup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, _ = exec.CommandContext(ctxCleanup, "systemctl", "stop", s.unitServiceName()).CombinedOutput()
	_, _ = exec.CommandContext(ctxCleanup, "systemctl", "reset-failed", s.unitServiceName()).CombinedOutput()
}

func (s *Service) unitServiceName() string {
	return s.cfg.UnitName + ".service"
}

func normalizeUnitName(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, ".service")
	return v
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'"
}

func parseInt(v string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(v))
	return n
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}
