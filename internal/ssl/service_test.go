package ssl

import (
	"context"
	"io"
	"log"
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

