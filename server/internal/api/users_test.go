package api

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestBcryptHashAndVerify proves that a password hashed by the directory
// constructor can be verified by Authenticate, and a wrong password fails.
// This is the anti-bluff baseline: a bcrypt PASS-bluff would be a "hash"
// that accepts any password or rejects every password.
func TestBcryptHashAndVerify(t *testing.T) {
	dir := NewStaticUserDirectory(StaticUser{
		Username: "alice@test.example",
		Password: "correct-horse-battery-staple",
		Roles:    []string{RoleAdmin},
	})

	// Verify correct password.
	roles, ok := dir.Authenticate("alice@test.example", "correct-horse-battery-staple")
	if !ok {
		t.Fatal("Authenticate with correct password must succeed")
	}
	if len(roles) != 1 || roles[0] != RoleAdmin {
		t.Fatalf("expected [admin], got %v", roles)
	}

	// Verify wrong password.
	_, ok = dir.Authenticate("alice@test.example", "wrong-password")
	if ok {
		t.Fatal("Authenticate with wrong password must fail")
	}

	// Verify unknown user.
	_, ok = dir.Authenticate("nonexistent@test.example", "correct-horse-battery-staple")
	if ok {
		t.Fatal("Authenticate with unknown user must fail")
	}
}

// TestBcryptCost proves passwords are hashed at cost 12.
func TestBcryptCost(t *testing.T) {
	dir := NewStaticUserDirectory(StaticUser{
		Username: "bob@test.example",
		Password: "s3cret",
		Roles:    []string{RoleViewer},
	})

	user, ok := dir.users["bob@test.example"]
	if !ok {
		t.Fatal("user must be present after construction")
	}

	cost, err := bcrypt.Cost([]byte(user.hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost != bcryptCost {
		t.Fatalf("want cost %d, got %d", bcryptCost, cost)
	}
}

// TestBcryptStoredAsHashProves the internal hash is NOT the original plaintext
// password. If this fails, we are still storing plaintext, which is the exact
// security defect OTA-036 exists to fix.
func TestBcryptStoredAsHashProves(t *testing.T) {
	const plaintext = "s3cret"
	dir := NewStaticUserDirectory(StaticUser{
		Username: "carol@test.example",
		Password: plaintext,
		Roles:    []string{RoleViewer},
	})

	user, ok := dir.users["carol@test.example"]
	if !ok {
		t.Fatal("user must be present after construction")
	}
	if user.hash == plaintext {
		t.Fatalf("FATAL: stored hash is the original plaintext password — this is the defect OTA-036 must prevent")
	}
	// The stored value must start with the bcrypt prefix.
	if len(user.hash) < 4 || user.hash[:4] != "$2a$" {
		t.Fatalf("stored hash does not look like a bcrypt hash: %q", user.hash[:min(len(user.hash), 16)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestEmptyPasswordSkipped proves that a user with an empty password is
// not added to the directory (startup safeguard).
func TestEmptyPasswordSkipped(t *testing.T) {
	dir := NewStaticUserDirectory(
		StaticUser{Username: "ok@test.example", Password: "s3cret", Roles: []string{RoleAdmin}},
		StaticUser{Username: "bad@test.example", Password: "", Roles: []string{RoleAdmin}},
	)

	// The ok user must authenticate.
	_, ok := dir.Authenticate("ok@test.example", "s3cret")
	if !ok {
		t.Fatal("user with valid password must authenticate")
	}

	// The empty-password user must NOT be present in the store.
	if _, exists := dir.users["bad@test.example"]; exists {
		t.Fatal("user with empty password must be skipped")
	}
}
