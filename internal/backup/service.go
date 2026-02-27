package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrInvalidBackupPath = errors.New("invalid backup path")

type Service struct {
	apply     bool
	stateFile string
	backupDir string
	logger    *log.Logger
}

type BackupResult struct {
	File      string    `json:"file"`
	CreatedAt time.Time `json:"created_at"`
}

func NewService(apply bool, stateFile, backupDir string, logger *log.Logger) *Service {
	return &Service{
		apply:     apply,
		stateFile: stateFile,
		backupDir: backupDir,
		logger:    logger,
	}
}

func (s *Service) Run(ctx context.Context) (BackupResult, error) {
	select {
	case <-ctx.Done():
		return BackupResult{}, ctx.Err()
	default:
	}

	if strings.TrimSpace(s.stateFile) == "" {
		return BackupResult{}, errors.New("state file is empty")
	}
	if strings.TrimSpace(s.backupDir) == "" {
		return BackupResult{}, errors.New("backup dir is empty")
	}

	now := time.Now().UTC()
	name := fmt.Sprintf("nusantara_state_%s.json", now.Format("20060102_150405"))
	target := filepath.Join(s.backupDir, name)

	if !s.apply {
		s.logf("dry-run backup run source=%s target=%s", s.stateFile, target)
		return BackupResult{File: target, CreatedAt: now}, nil
	}

	if err := os.MkdirAll(s.backupDir, 0o750); err != nil {
		return BackupResult{}, fmt.Errorf("create backup dir: %w", err)
	}
	if err := copyFile(s.stateFile, target, 0o640); err != nil {
		return BackupResult{}, err
	}
	return BackupResult{File: target, CreatedAt: now}, nil
}

func (s *Service) Restore(ctx context.Context, backupFile string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	backupFile = strings.TrimSpace(backupFile)
	if backupFile == "" {
		return ErrInvalidBackupPath
	}
	absBackupDir, err := filepath.Abs(s.backupDir)
	if err != nil {
		return ErrInvalidBackupPath
	}
	absFile, err := filepath.Abs(backupFile)
	if err != nil {
		return ErrInvalidBackupPath
	}
	if !strings.HasPrefix(absFile, absBackupDir+string(os.PathSeparator)) && absFile != absBackupDir {
		return ErrInvalidBackupPath
	}

	if !s.apply {
		s.logf("dry-run backup restore from=%s to=%s", absFile, s.stateFile)
		return nil
	}

	if _, err := os.Stat(absFile); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}
	if err := copyFile(absFile, s.stateFile, 0o640); err != nil {
		return err
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("move backup file: %w", err)
	}
	return nil
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

