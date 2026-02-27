package provision

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"nusantara/internal/store"
)

type NginxConfig struct {
	Apply         bool
	AvailableDir  string
	EnabledDir    string
	TestCommand   string
	ReloadCommand string
}

type NginxProvisioner struct {
	cfg    NginxConfig
	logger *log.Logger
}

func NewNginxProvisioner(cfg NginxConfig, logger *log.Logger) *NginxProvisioner {
	return &NginxProvisioner{
		cfg:    cfg,
		logger: logger,
	}
}

func (p *NginxProvisioner) ProvisionSite(ctx context.Context, site store.Site) error {
	if !p.cfg.Apply {
		p.logf("dry-run provisioning site=%s domain=%s", site.ID, site.Domain)
		return nil
	}

	if site.Domain == "" {
		return errors.New("site domain is empty")
	}
	if site.RootPath == "" {
		return errors.New("site root_path is empty")
	}

	if err := os.MkdirAll(p.cfg.AvailableDir, 0o755); err != nil {
		return fmt.Errorf("create available dir: %w", err)
	}
	if err := os.MkdirAll(p.cfg.EnabledDir, 0o755); err != nil {
		return fmt.Errorf("create enabled dir: %w", err)
	}
	if err := os.MkdirAll(site.RootPath, 0o755); err != nil {
		return fmt.Errorf("create site root: %w", err)
	}
	if err := ensureRuntimeBootstrap(site); err != nil {
		return fmt.Errorf("bootstrap site root: %w", err)
	}

	confName := sanitizeConfName(site.Domain) + ".conf"
	confPath := filepath.Join(p.cfg.AvailableDir, confName)
	linkPath := filepath.Join(p.cfg.EnabledDir, confName)

	previousConf, hadPreviousConf, err := readIfExists(confPath)
	if err != nil {
		return fmt.Errorf("read previous conf: %w", err)
	}
	previousLinkTarget, hadPreviousLink, err := readLinkIfExists(linkPath)
	if err != nil {
		return fmt.Errorf("read previous link: %w", err)
	}

	if err := writeAtomic(confPath, []byte(renderNginxServer(site))); err != nil {
		return fmt.Errorf("write nginx conf: %w", err)
	}
	if err := upsertSymlink(confPath, linkPath); err != nil {
		_ = rollback(confPath, linkPath, previousConf, hadPreviousConf, previousLinkTarget, hadPreviousLink)
		return fmt.Errorf("upsert symlink: %w", err)
	}

	if err := runCommand(ctx, p.cfg.TestCommand); err != nil {
		_ = rollback(confPath, linkPath, previousConf, hadPreviousConf, previousLinkTarget, hadPreviousLink)
		return fmt.Errorf("nginx test failed: %w", err)
	}
	if err := runCommand(ctx, p.cfg.ReloadCommand); err != nil {
		_ = rollback(confPath, linkPath, previousConf, hadPreviousConf, previousLinkTarget, hadPreviousLink)
		return fmt.Errorf("nginx reload failed: %w", err)
	}

	p.logf("site provisioned domain=%s conf=%s", site.Domain, confPath)
	return nil
}

func (p *NginxProvisioner) DeprovisionSite(ctx context.Context, site store.Site) error {
	if !p.cfg.Apply {
		p.logf("dry-run deprovision site=%s domain=%s", site.ID, site.Domain)
		return nil
	}
	if site.Domain == "" {
		return errors.New("site domain is empty")
	}

	confName := sanitizeConfName(site.Domain) + ".conf"
	confPath := filepath.Join(p.cfg.AvailableDir, confName)
	linkPath := filepath.Join(p.cfg.EnabledDir, confName)

	if err := os.Remove(linkPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove symlink: %w", err)
	}
	if err := os.Remove(confPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove conf: %w", err)
	}

	if err := runCommand(ctx, p.cfg.TestCommand); err != nil {
		return fmt.Errorf("nginx test failed: %w", err)
	}
	if err := runCommand(ctx, p.cfg.ReloadCommand); err != nil {
		return fmt.Errorf("nginx reload failed: %w", err)
	}

	p.logf("site deprovisioned domain=%s", site.Domain)
	return nil
}

func sanitizeConfName(domain string) string {
	clean := strings.TrimSpace(strings.ToLower(domain))
	clean = strings.ReplaceAll(clean, "..", ".")
	return strings.ReplaceAll(clean, "/", "-")
}

func renderNginxServer(site store.Site) string {
	serverCore := runtimeServerCore(site.Runtime, site.RootPath)
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    root %s;
    index index.php index.html index.htm;

%s
}
`, site.Domain, site.RootPath, serverCore)
}

func runtimeServerCore(runtimeName, rootPath string) string {
	switch runtimeName {
	case "node":
		return `    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }`
	case "python":
		return `    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }`
	case "static":
		return `    location / {
        try_files $uri $uri/ =404;
    }`
	default:
		return fmt.Sprintf(`    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME %s$fastcgi_script_name;
        fastcgi_pass unix:/run/php/php8.1-fpm.sock;
        fastcgi_index index.php;
    }`, rootPath)
	}
}

func runCommand(ctx context.Context, raw string) error {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return errors.New("empty command")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", raw, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writeAtomic(path string, content []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func upsertSymlink(target, linkPath string) error {
	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			currentTarget, readErr := os.Readlink(linkPath)
			if readErr == nil && currentTarget == target {
				return nil
			}
		}
		if err := os.Remove(linkPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Symlink(target, linkPath)
}

func readIfExists(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return content, true, nil
}

func readLinkIfExists(path string) (string, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", true, nil
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", false, err
	}
	return target, true, nil
}

func rollback(confPath, linkPath string, previousConf []byte, hadPreviousConf bool, previousLinkTarget string, hadPreviousLink bool) error {
	if hadPreviousConf {
		if err := writeAtomic(confPath, previousConf); err != nil {
			return err
		}
	} else if err := os.Remove(confPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if !hadPreviousLink {
		if err := os.Remove(linkPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := os.Remove(linkPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if previousLinkTarget == "" {
		return nil
	}
	return os.Symlink(previousLinkTarget, linkPath)
}

func (p *NginxProvisioner) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Printf(format, args...)
	}
}

func ensureRuntimeBootstrap(site store.Site) error {
	root := strings.TrimSpace(site.RootPath)
	if root == "" {
		return errors.New("empty root path")
	}

	runtimeName := strings.ToLower(strings.TrimSpace(site.Runtime))
	switch runtimeName {
	case "php":
		exists, err := hasAnyIndex(root)
		if err != nil || exists {
			return err
		}
		indexPath := filepath.Join(root, "index.php")
		return writeFileIfNotExists(indexPath, []byte(defaultPHPIndex(site.Domain)))
	case "static":
		exists, err := hasAnyIndex(root)
		if err != nil || exists {
			return err
		}
		indexPath := filepath.Join(root, "index.html")
		return writeFileIfNotExists(indexPath, []byte(defaultStaticIndex(site.Domain)))
	default:
		return nil
	}
}

func hasAnyIndex(root string) (bool, error) {
	for _, name := range []string{"index.php", "index.html", "index.htm"} {
		p := filepath.Join(root, name)
		_, err := os.Stat(p)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

func writeFileIfNotExists(path string, content []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}

func defaultStaticIndex(domain string) string {
	label := strings.TrimSpace(domain)
	if label == "" {
		label = "site"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
</head>
<body>
  <h1>%s is live</h1>
  <p>Provisioned by Nusantara Panel.</p>
</body>
</html>
`, label, label)
}

func defaultPHPIndex(domain string) string {
	label := strings.TrimSpace(domain)
	if label == "" {
		label = "site"
	}
	return fmt.Sprintf(`<?php
header('Content-Type: text/html; charset=utf-8');
?>
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
</head>
<body>
  <h1>%s is live</h1>
  <p>Provisioned by Nusantara Panel (PHP runtime).</p>
</body>
</html>
`, label, label)
}
