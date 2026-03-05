package requestflow

import "time"

func runCleanupLoop(stop <-chan struct{}, cleanup func()) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cleanup()
		case <-stop:
			return
		}
	}
}
