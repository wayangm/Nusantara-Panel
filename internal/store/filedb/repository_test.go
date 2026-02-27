package filedb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"nusantara/internal/store"
)

func TestUserLifecycle(t *testing.T) {
	tmp := t.TempDir()
	repo, err := New(filepath.Join(tmp, "state.json"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	err = repo.CreateUser(ctx, store.User{
		ID:           "u1",
		Username:     "admin",
		PasswordHash: "hash",
		Role:         store.RoleAdmin,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, err := repo.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.ID != "u1" {
		t.Fatalf("unexpected user id: %s", got.ID)
	}
}

