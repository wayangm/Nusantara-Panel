package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidDatabaseName = errors.New("invalid database name")
	ErrInvalidUsername     = errors.New("invalid database username")
	ErrInvalidPassword     = errors.New("invalid database password")
	ErrInvalidHost         = errors.New("invalid database host")
)

var (
	identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	hostPattern       = regexp.MustCompile(`^[a-zA-Z0-9._%-]+$`)
	systemDatabases   = map[string]struct{}{
		"mysql":              {},
		"information_schema": {},
		"performance_schema": {},
		"sys":                {},
	}
)

type Service struct {
	apply        bool
	mysqlCommand string
	timeout      time.Duration
	logger       *log.Logger
}

type DatabaseInfo struct {
	Name   string `json:"name"`
	System bool   `json:"system"`
}

type CreateUserInput struct {
	Database string
	Username string
	Password string
	Host     string
}

func NewService(apply bool, mysqlCommand string, timeout time.Duration, logger *log.Logger) *Service {
	if strings.TrimSpace(mysqlCommand) == "" {
		mysqlCommand = "mysql"
	}
	if timeout < time.Second {
		timeout = 10 * time.Second
	}
	return &Service{
		apply:        apply,
		mysqlCommand: mysqlCommand,
		timeout:      timeout,
		logger:       logger,
	}
}

func (s *Service) ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	if !s.apply {
		return []DatabaseInfo{}, nil
	}

	lines, err := s.query(ctx, "SHOW DATABASES;")
	if err != nil {
		return nil, err
	}

	items := make([]DatabaseInfo, 0, len(lines))
	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		_, system := systemDatabases[strings.ToLower(name)]
		items = append(items, DatabaseInfo{
			Name:   name,
			System: system,
		})
	}
	return items, nil
}

func (s *Service) CreateDatabase(ctx context.Context, database string) error {
	database = strings.TrimSpace(database)
	if !isValidIdentifier(database) {
		return ErrInvalidDatabaseName
	}
	sql := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		database,
	)
	return s.execSQL(ctx, sql)
}

func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) error {
	database := strings.TrimSpace(input.Database)
	username := strings.TrimSpace(input.Username)
	password := input.Password
	host := strings.TrimSpace(input.Host)
	if host == "" {
		host = "localhost"
	}

	switch {
	case !isValidIdentifier(database):
		return ErrInvalidDatabaseName
	case !isValidIdentifier(username):
		return ErrInvalidUsername
	case len(strings.TrimSpace(password)) < 8:
		return ErrInvalidPassword
	case !hostPattern.MatchString(host):
		return ErrInvalidHost
	}

	sql := strings.Join([]string{
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';", escapeSQLString(username), escapeSQLString(host), escapeSQLString(password)),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s';", database, escapeSQLString(username), escapeSQLString(host)),
		"FLUSH PRIVILEGES;",
	}, " ")
	return s.execSQL(ctx, sql)
}

func (s *Service) execSQL(ctx context.Context, sql string) error {
	if !s.apply {
		s.logf("dry-run mysql exec sql=%s", sql)
		return nil
	}
	_, err := s.run(ctx, "-Nse", sql)
	return err
}

func (s *Service) query(ctx context.Context, sql string) ([]string, error) {
	out, err := s.run(ctx, "-Nse", sql)
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(out)
	if raw == "" {
		return []string{}, nil
	}
	return strings.Split(raw, "\n"), nil
}

func (s *Service) run(ctx context.Context, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, s.mysqlCommand, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %v: %w (%s)", s.mysqlCommand, args, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func isValidIdentifier(name string) bool {
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	return identifierPattern.MatchString(name)
}

func escapeSQLString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

