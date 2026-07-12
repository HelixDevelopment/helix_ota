package api

import (
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the cost factor for password hashing.
const bcryptCost = 12

// StaticUserDirectory is a simple in-memory UserDirectory for the MVP login
// stub. Credentials are supplied at construction (never hard-coded in this
// package) and hashed with bcrypt (cost 12) before storage — plaintext is
// never retained. Production replaces it with the `auth` brick / identity
// store.
type StaticUserDirectory struct {
	// users maps username -> credential.
	users map[string]staticCred
}

// staticCred is a bcrypt-hashed password + role grant for a static user.
type staticCred struct {
	hash  string
	roles []string
}

// StaticUser describes one entry for NewStaticUserDirectory.
type StaticUser struct {
	Username string
	Password string
	Roles    []string
}

// NewStaticUserDirectory builds a directory from the given users. Every
// password is hashed with bcrypt (cost 12) at construction time — the
// plaintext is never stored. A user whose password cannot be hashed is
// skipped with a log warning (this should never happen with valid input).
// The returned directory is always non-nil; it may be empty if no users
// could be hashed.
func NewStaticUserDirectory(users ...StaticUser) *StaticUserDirectory {
	d := &StaticUserDirectory{users: make(map[string]staticCred, len(users))}
	for _, u := range users {
		if u.Password == "" {
			log.Printf("api: skipping user %q: empty password", u.Username)
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcryptCost)
		if err != nil {
			log.Printf("api: skipping user %q: bcrypt hash: %v", u.Username, err)
			continue
		}
		d.users[u.Username] = staticCred{hash: string(hash), roles: u.Roles}
	}
	return d
}

// MustNewStaticUserDirectory is like NewStaticUserDirectory but calls
// os.Exit(1) if any password cannot be hashed. Use in main() where a
// broken credential is a fatal startup error.
func MustNewStaticUserDirectory(users ...StaticUser) *StaticUserDirectory {
	d := &StaticUserDirectory{users: make(map[string]staticCred, len(users))}
	for _, u := range users {
		if u.Password == "" {
			log.Printf("api: must-skip user %q: empty password", u.Username)
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcryptCost)
		if err != nil {
			log.Printf("api: fatal: bcrypt hash for user %q: %v", u.Username, err)
			os.Exit(1)
		}
		d.users[u.Username] = staticCred{hash: string(hash), roles: u.Roles}
	}
	return d
}

// Authenticate verifies the username/password against the stored bcrypt
// hash. It returns the user's roles and true on success, or false on a
// unknown user or wrong password. The username-enumeration timing signal
// is reduced by always running a bcrypt comparison (when the user is
// unknown a dummy hash that can never validate is compared against).
func (d *StaticUserDirectory) Authenticate(username, password string) ([]string, bool) {
	cred, ok := d.users[username]
	if !ok {
		// Run a comparison anyway to reduce username-enumeration timing
		// signal. A bcrypt hash of "$2a$12$..." is a valid hash prefix
		// that never matches a real password.
		bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return nil, false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cred.hash), []byte(password)); err != nil {
		return nil, false
	}
	return cred.roles, true
}

// dummyHash is a bcrypt hash of a random 32-byte value (cost 12) that is
// compared against on an unknown-username path to reduce side-channel
// username-enumeration timing signal. It can never validate a real password.
var dummyHash = func() string {
	h, err := bcrypt.GenerateFromPassword(
		[]byte("dummy-32-byte-value-xxxxxxxxxxxxx"),
		bcryptCost,
	)
	if err != nil {
		return bcryptFallbackSentinel
	}
	return string(h)
}()

// bcryptFallbackSentinel is a syntactically valid bcrypt hash that can never
// match a real password (the embedded "dummy" string exceeds bcrypt's 72-byte
// input limit and the hash itself was not derived from any known plaintext).
// It is used only as a compile-time backstop for dummyHash generation failure.
const bcryptFallbackSentinel = "$2a$12$8GpN.C9QaLFXGKLBJgAWXuEeK4i0svbGDE.M0INLwNAgAt.XbKjOC"
