package provision

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nusantara/internal/store"
)

func TestProvisionDryRun(t *testing.T) {
	p := NewNginxProvisioner(NginxConfig{
		Apply: false,
	}, log.New(io.Discard, "", 0))

	site := store.Site{
		ID:       "site-1",
		Domain:   "example.com",
		RootPath: "/var/www/example",
		Runtime:  "php",
	}
	err := p.ProvisionSite(context.Background(), site)
	if err != nil {
		t.Fatalf("dry run should not fail: %v", err)
	}
	if err := p.DeprovisionSite(context.Background(), site); err != nil {
		t.Fatalf("dry run deprovision should not fail: %v", err)
	}
}

func TestRenderNginxServer(t *testing.T) {
	conf := renderNginxServer(store.Site{
		Domain:   "example.com",
		RootPath: "/var/www/example",
		Runtime:  "static",
	})
	if conf == "" {
		t.Fatalf("empty config")
	}
	if !strings.Contains(conf, "server_name example.com;") {
		t.Fatalf("missing server_name")
	}
	if !strings.Contains(conf, "try_files $uri $uri/ =404;") {
		t.Fatalf("missing static location")
	}
}

func TestEnsureRuntimeBootstrapStaticCreatesIndex(t *testing.T) {
	root := t.TempDir()
	site := store.Site{
		Domain:   "site.example.test",
		RootPath: root,
		Runtime:  "static",
	}

	if err := ensureRuntimeBootstrap(site); err != nil {
		t.Fatalf("bootstrap should succeed: %v", err)
	}

	indexPath := filepath.Join(root, "index.html")
	b, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index file: %v", err)
	}
	if !strings.Contains(string(b), "site.example.test is live") {
		t.Fatalf("index content should include domain label")
	}
}

func TestEnsureRuntimeBootstrapDoesNotOverwriteExistingIndex(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.php")
	if err := os.WriteFile(indexPath, []byte("custom"), 0o644); err != nil {
		t.Fatalf("seed custom index: %v", err)
	}

	site := store.Site{
		Domain:   "site.example.test",
		RootPath: root,
		Runtime:  "php",
	}
	if err := ensureRuntimeBootstrap(site); err != nil {
		t.Fatalf("bootstrap should succeed: %v", err)
	}

	b, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read existing index: %v", err)
	}
	if string(b) != "custom" {
		t.Fatalf("existing index must not be overwritten")
	}
}
