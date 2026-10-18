package tunnel

// ReadyTimeout lets tests shorten how long Open waits for the pod.
var ReadyTimeout = &readyTimeout

// ReconnectDelay lets tests skip the pause before re-forwarding.
var ReconnectDelay = &reconnectDelay
