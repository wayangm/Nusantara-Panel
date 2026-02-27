package db

import (
	"context"
	"io"
	"log"
	"testing"
	"time"
)

func TestCreateDatabaseValidation(t *testing.T) {
	svc := NewService(false, "mysql", 2*time.Second, log.New(io.Discard, "", 0))
	if err := svc.CreateDatabase(context.Background(), "app_db"); err != nil {
		t.Fatalf("expected valid database: %v", err)
	}
	if err := svc.CreateDatabase(context.Background(), "bad-name"); err == nil {
		t.Fatalf("expected invalid name")
	}
}

func TestCreateUserValidation(t *testing.T) {
	svc := NewService(false, "mysql", 2*time.Second, nil)
	err := svc.CreateUser(context.Background(), CreateUserInput{
		Database: "app_db",
		Username: "app_user",
		Password: "StrongPass123",
		Host:     "localhost",
	})
	if err != nil {
		t.Fatalf("expected valid user input: %v", err)
	}

	if err := svc.CreateUser(context.Background(), CreateUserInput{
		Database: "app_db",
		Username: "bad-user",
		Password: "StrongPass123",
		Host:     "localhost",
	}); err == nil {
		t.Fatalf("expected invalid username")
	}
}

