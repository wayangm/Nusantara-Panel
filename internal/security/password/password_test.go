package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("StrongPass123")
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	if !Verify("StrongPass123", hash) {
		t.Fatalf("expected verify to pass")
	}
	if Verify("wrong-pass", hash) {
		t.Fatalf("expected verify to fail")
	}
}

func TestWeakPassword(t *testing.T) {
	if _, err := Hash("short"); err == nil {
		t.Fatalf("expected weak password error")
	}
}

