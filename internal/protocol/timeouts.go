package protocol

import "time"

// A cold install downloads a large, hash-pinned executable. Keep the download,
// operation and lease budgets ordered so a slow transfer leaves time to install
// and acknowledge its result without another task taking over the machine.
const (
	RuntimeDownloadTimeout = 8 * time.Minute
	DeploymentTimeout      = 9 * time.Minute
	DeploymentLease        = 10 * time.Minute
	RuntimeResponseTimeout = 9 * time.Minute
)
