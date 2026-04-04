package web

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/user/vocabgen/internal/update"
)

// updateChecker manages background and on-demand update checks.
type updateChecker struct {
	mu         sync.RWMutex
	info       *update.UpdateInfo
	dismissed  bool
	currentVer string
	logger     *slog.Logger
}

// newUpdateChecker creates an updateChecker for the given compiled-in version.
func newUpdateChecker(version string, logger *slog.Logger) *updateChecker {
	return &updateChecker{
		currentVer: version,
		logger:     logger,
	}
}

// cached returns the cached update check result, or nil if no check has completed.
func (uc *updateChecker) cached() *update.UpdateInfo {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.info
}

// dismiss marks the update banner as dismissed for this server session.
func (uc *updateChecker) dismiss() {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.dismissed = true
}

// isDismissed reports whether the update banner has been dismissed.
func (uc *updateChecker) isDismissed() bool {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.dismissed
}

// startBackground launches a background goroutine that performs an update check
// with a 5-second timeout. It does not block the caller.
func (uc *updateChecker) startBackground(ctx context.Context) {
	go func() {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		uc.checkNow(checkCtx)
	}()
}

// checkNow delegates to the shared update package, caches the result, and returns it.
func (uc *updateChecker) checkNow(ctx context.Context) *update.UpdateInfo {
	info := update.CheckNow(ctx, uc.currentVer)
	uc.cacheResult(info)
	return info
}

// cacheResult stores the update info under the write lock.
func (uc *updateChecker) cacheResult(info *update.UpdateInfo) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.info = info
	uc.logger.Info("update check complete",
		"current", info.CurrentVersion,
		"latest", info.LatestVersion,
		"hasUpdate", info.HasUpdate,
		"error", info.Error,
	)
}
