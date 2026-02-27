package sites

import (
	"os"
	"path/filepath"
	"testing"

	"nusantara/internal/store"
)

func TestIsValidDomain(t *testing.T) {
	cases := []struct {
		domain string
		ok     bool
	}{
		{domain: "example.com", ok: true},
		{domain: "sub.example.com", ok: true},
		{domain: "-bad.example.com", ok: false},
		{domain: "bad..example.com", ok: false},
		{domain: "localhost", ok: false},
	}
	for _, tc := range cases {
		if got := isValidDomain(tc.domain); got != tc.ok {
			t.Fatalf("domain %q expected %t got %t", tc.domain, tc.ok, got)
		}
	}
}

func TestIsValidRootPath(t *testing.T) {
	cases := []struct {
		root string
		ok   bool
	}{
		{root: "/var/www/site", ok: true},
		{root: "/", ok: false},
		{root: "relative/path", ok: false},
	}
	for _, tc := range cases {
		if got := isValidRootPath(tc.root); got != tc.ok {
			t.Fatalf("root %q expected %t got %t", tc.root, tc.ok, got)
		}
	}
}

func TestResolveEditableFileDefaultsByRuntime(t *testing.T) {
	sitePHP := store.Site{Runtime: "php", RootPath: t.TempDir()}
	file, err := resolveEditableFile(sitePHP, "", false)
	if err != nil {
		t.Fatalf("resolve php default: %v", err)
	}
	if file != "index.php" {
		t.Fatalf("expected index.php, got %s", file)
	}

	siteStatic := store.Site{Runtime: "static", RootPath: t.TempDir()}
	file, err = resolveEditableFile(siteStatic, "", false)
	if err != nil {
		t.Fatalf("resolve static default: %v", err)
	}
	if file != "index.html" {
		t.Fatalf("expected index.html, got %s", file)
	}
}

func TestResolveEditableFileRejectsInvalidName(t *testing.T) {
	site := store.Site{Runtime: "php", RootPath: t.TempDir()}
	if _, err := resolveEditableFile(site, "app.js", false); err == nil {
		t.Fatalf("expected invalid file error")
	}
}

func TestResolveEditableFilePrefersExistingIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.htm"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("seed index.htm: %v", err)
	}
	site := store.Site{Runtime: "static", RootPath: root}
	file, err := resolveEditableFile(site, "", true)
	if err != nil {
		t.Fatalf("resolve existing: %v", err)
	}
	if file != "index.htm" {
		t.Fatalf("expected existing index.htm, got %s", file)
	}
}

func TestNormalizeRelativePath(t *testing.T) {
	pathValue, err := normalizeRelativePath("assets/logo.png", false)
	if err != nil {
		t.Fatalf("normalize valid path: %v", err)
	}
	if pathValue != "assets/logo.png" {
		t.Fatalf("unexpected normalized path: %s", pathValue)
	}

	if _, err := normalizeRelativePath("/etc/passwd", false); err == nil {
		t.Fatalf("expected absolute path rejection")
	}
	if _, err := normalizeRelativePath("../secret.txt", false); err == nil {
		t.Fatalf("expected traversal path rejection")
	}
}

func TestIsWithinRoot(t *testing.T) {
	root := filepath.Clean("/var/www/site1")
	if !isWithinRoot(root, filepath.Join(root, "assets", "logo.png")) {
		t.Fatalf("expected path inside root")
	}
	if isWithinRoot(root, "/var/www/site1-backup/file.txt") {
		t.Fatalf("expected path outside root to be rejected")
	}
}
