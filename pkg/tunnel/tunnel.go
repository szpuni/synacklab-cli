// Package tunnel opens TCP tunnels to private network targets through a
// short-lived socat pod and the Kubernetes port-forward API.
package tunnel

import "time"

// DefaultMaxDuration is how long a tunnel pod may live before Kubernetes kills it.
const DefaultMaxDuration = 8 * time.Hour
