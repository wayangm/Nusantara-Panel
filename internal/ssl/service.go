package ssl

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
	ErrInvalidDomain = errors.New("invalid domain")
	ErrInvalidEmail  = errors.New("invalid email")
)

var (
	domainPattern = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	emailPattern  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

type Service struct {
	apply          bool
	certbotCommand string
	timeout        time.Duration
	logger         *log.Logger
}

func NewService(apply bool, certbotCommand string, timeout time.Duration, logger *log.Logger) *Service {
	if strings.TrimSpace(certbotCommand) == "" {
		certbotCommand = "certbot"
	}
	if timeout < 10*time.Second {
		timeout = 2 * time.Minute
	}
	return &Service{
		apply:          apply,
		certbotCommand: certbotCommand,
		timeout:        timeout,
		logger:         logger,
	}
}

func (s *Service) Issue(ctx context.Context, domain, email string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	email = strings.TrimSpace(email)
	if !isValidDomain(domain) {
		return ErrInvalidDomain
	}
	if !emailPattern.MatchString(email) {
		return ErrInvalidEmail
	}

	if !s.apply {
		s.logf("dry-run ssl issue domain=%s email=%s", domain, email)
		return nil
	}

	runCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	args := []string{
		"--nginx",
		"-d", domain,
		"--non-interactive",
		"--agree-tos",
		"-m", email,
		"--redirect",
	}
	if err := runCommand(runCtx, s.certbotCommand, args...); err != nil {
		return fmt.Errorf("issue cert: %w", err)
	}
	return nil
}

func (s *Service) Renew(ctx context.Context) error {
	if !s.apply {
		s.logf("dry-run ssl renew")
		return nil
	}

	runCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if err := runCommand(runCtx, s.certbotCommand, "renew", "--non-interactive"); err != nil {
		return fmt.Errorf("renew cert: %w", err)
	}
	return nil
}

func isValidDomain(domain string) bool {
	if len(domain) < 3 || len(domain) > 253 {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return false
	}
	if !domainPattern.MatchString(domain) {
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

func runCommand(ctx context.Context, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", command, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

