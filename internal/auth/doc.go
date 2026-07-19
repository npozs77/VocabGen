// Package auth provides stateless API-key authentication for service-accounts.
// It loads service-account definitions from a users.yaml file (bcrypt-hashed
// keys with scope) and exposes HTTP middleware that validates Bearer tokens
// against the configured accounts. When no users.yaml is present, all requests
// pass through unauthenticated (backward-compatible open access).
package auth
