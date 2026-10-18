package tunnel

// ReadyTimeout lets tests shorten how long Open waits for the pod.
var ReadyTimeout = &readyTimeout

// RetryInterval lets tests skip the pause between retries.
var RetryInterval = &retryInterval

// DeleteTimeout lets tests shorten how long teardown retries deleting the pod.
var DeleteTimeout = &deleteTimeout

// PodWatchInterval lets tests check the pod state more often while forwarding.
var PodWatchInterval = &podWatchInterval
