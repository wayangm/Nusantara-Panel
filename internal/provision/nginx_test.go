package provision

import (
	"context"
	"io"
	"log"
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

