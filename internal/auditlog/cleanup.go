package auditlog

import "time"

// NOTE: If more shared store functions are added beyond cleanup,
// consider renaming this file to store_common.go or similar.

// CleanupInterval is how often the cleanup goroutine runs to delete old log entries.
const CleanupInterval = 1 * time.Hour

// RunCleanupLoop runs a cleanup function periodically until the stop channel is closed.
// It runs cleanup immediately on start, then at CleanupInterval intervals.
func RunCleanupLoop(stop <-chan struct{}, cleanupFn func()) {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	// Run initial cleanup
	cleanupFn()

	for {
		select {
		case <-ticker.C:
			cleanupFn()
		case <-stop:
			return
		}
	}
}
