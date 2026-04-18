// Package service implements the business logic for vocabulary lookups and
// batch processing, shared by both the CLI and web UI layers. It orchestrates
// normalization, cache checks, LLM invocation, quality checks, conflict
// resolution, and database persistence.
package service
