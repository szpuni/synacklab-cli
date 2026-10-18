package tunnel

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// readyTimeout bounds the wait for scheduling, image pull, and socat start.
var readyTimeout = 2 * time.Minute

// waitingFailures are container waiting reasons that will not resolve by waiting.
var waitingFailures = map[string]bool{
	"ErrImagePull":               true,
	"ImagePullBackOff":           true,
	"InvalidImageName":           true,
	"CreateContainerConfigError": true,
}

// waitReady polls the pod until its container is ready, failing fast when the
// pod reports a reason it will never become ready.
func waitReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	var last *corev1.Pod
	err := wait.PollUntilContextTimeout(ctx, time.Second, readyTimeout, true, func(ctx context.Context) (bool, error) {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		last = pod
		return podReady(pod)
	})
	if err == nil {
		return nil
	}
	if wait.Interrupted(err) && ctx.Err() == nil {
		return fmt.Errorf("tunnel pod %s not ready after %s (%s)", name, readyTimeout, describe(last))
	}
	return fmt.Errorf("tunnel pod %s failed to start: %w", name, err)
}

func podReady(pod *corev1.Pod) (bool, error) {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason == corev1.PodReasonUnschedulable {
			return false, fmt.Errorf("%s: %s", cond.Reason, cond.Message)
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if w := cs.State.Waiting; w != nil && waitingFailures[w.Reason] {
			return false, fmt.Errorf("%s: %s", w.Reason, w.Message)
		}
		if t := cs.State.Terminated; t != nil {
			return false, fmt.Errorf("%s: socat exited with exit code %d %s", t.Reason, t.ExitCode, t.Message)
		}
	}
	switch pod.Status.Phase {
	case corev1.PodFailed, corev1.PodSucceeded:
		return false, errors.New(describe(pod))
	case corev1.PodRunning:
		return len(pod.Status.ContainerStatuses) == 1 && pod.Status.ContainerStatuses[0].Ready, nil
	}
	return false, nil
}

// describe summarises the pod's phase and reason for error messages.
func describe(pod *corev1.Pod) string {
	if pod == nil {
		return "pod status unknown"
	}
	s := "phase " + string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		s += ", reason " + pod.Status.Reason
	}
	if pod.Status.Message != "" {
		s += ": " + pod.Status.Message
	}
	return s
}
