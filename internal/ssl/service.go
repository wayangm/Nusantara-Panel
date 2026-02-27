package ssl

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

	_ = os.RemoveAll("/var/lib/letsencrypt/temp_checkpoint")

	issueArgs := issueCommandArgs(domain, email)
	if err := runCommand(runCtx, s.certbotCommand, issueArgs...); err != nil {
		if !certificateExists(domain) {
			return fmt.Errorf("issue cert: %w", err)
		}
		s.logf("cert issue command returned non-zero, but certificate already exists domain=%s err=%v", domain, err)
	}

	installArgs := []string{
		"install",
		"--cert-name", domain,
		"--nginx",
		"--redirect",
		"--non-interactive",
	}
	if err := runCommand(runCtx, s.certbotCommand, installArgs...); err != nil {
		return fmt.Errorf("install cert: %w", err)
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

func issueCommandArgs(domain, email string) []string {
	args := []string{
		"certonly",
		"-d", domain,
		"--non-interactive",
		"--agree-tos",
		"-m", email,
	}

	webroot := defaultWebrootForDomain(domain)
	if webroot == "" {
		return append(args, "--nginx")
	}
	return append(args, "--webroot", "-w", webroot)
}

func defaultWebrootForDomain(domain string) string {
	root := filepath.Join("/var/www", domain, "public")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return ""
	}
	return root
}

func certificateExists(domain string) bool {
	fullchain := filepath.Join("/etc/letsencrypt/live", domain, "fullchain.pem")
	privkey := filepath.Join("/etc/letsencrypt/live", domain, "privkey.pem")
	if _, err := os.Stat(fullchain); err != nil {
		return false
	}
	if _, err := os.Stat(privkey); err != nil {
		return false
	}
	return true
}
