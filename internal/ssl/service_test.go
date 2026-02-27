package ssl

import (
	"context"
	"io"
	"log"
	"strings"
	"testing"
	"time"
)

func TestIssueDryRun(t *testing.T) {
	svc := NewService(false, "certbot", 30*time.Second, log.New(io.Discard, "", 0))
	if err := svc.Issue(context.Background(), "example.com", "admin@example.com"); err != nil {
		t.Fatalf("issue dry-run failed: %v", err)
	}
}

func TestIssueValidation(t *testing.T) {
	svc := NewService(false, "certbot", 30*time.Second, nil)
	if err := svc.Issue(context.Background(), "bad domain", "admin@example.com"); err == nil {
		t.Fatalf("expected invalid domain error")
	}
	if err := svc.Issue(context.Background(), "example.com", "invalid-email"); err == nil {
		t.Fatalf("expected invalid email error")
	}
}

func TestIssueCommandArgsFallbackToNginxWhenNoWebroot(t *testing.T) {
	args := issueCommandArgs("unlikely-domain-for-test-1234567890.example", "admin@example.com")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--nginx") {
		t.Fatalf("expected --nginx fallback args, got: %s", joined)
	}
	if strings.Contains(joined, "--webroot") {
		t.Fatalf("did not expect --webroot in fallback args, got: %s", joined)
	}
}
