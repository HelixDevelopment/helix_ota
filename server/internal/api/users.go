package api

import "crypto/subtle"

// StaticUserDirectory is a simple in-memory UserDirectory for the MVP login
// stub. Credentials are supplied at construction (never hard-coded in this
// package); production replaces it with the `auth` brick / identity store.
type StaticUserDirectory struct {
	// users maps username -> credential.
	users map[string]staticCred
}

// staticCred is a password + role grant for a static user.
type staticCred struct {
	password string
	roles    []string
}

// StaticUser describes one entry for NewStaticUserDirectory.
type StaticUser struct {
	Username string
	Password string
	Roles    []string
}

// NewStaticUserDirectory builds a directory from the given users.
func NewStaticUserDirectory(users ...StaticUser) *StaticUserDirectory {
	d := &StaticUserDirectory{users: make(map[string]staticCred, len(users))}
	for _, u := range users {
		d.users[u.Username] = staticCred{password: u.Password, roles: u.Roles}
	}
	return d
}

// Authenticate verifies the username/password with a constant-time password
// compare and returns the user's roles.
func (d *StaticUserDirectory) Authenticate(username, password string) ([]string, bool) {
	cred, ok := d.users[username]
	if !ok {
		// Run a comparison anyway to reduce username-enumeration timing signal.
		subtle.ConstantTimeCompare([]byte(password), []byte(password))
		return nil, false
	}
	if subtle.ConstantTimeCompare([]byte(cred.password), []byte(password)) != 1 {
		return nil, false
	}
	return cred.roles, true
}
