// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventenrichprocessor // import "github.com/openchoreo/community-modules/observability-events-otel-collector/k8seventenrichprocessor"

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
)

// objectGetter fetches an object from an informer cache by namespace and name.
// It returns a metav1.Object so the caller can read labels/annotations
// generically regardless of the concrete kind.
type objectGetter func(namespace, name string) (metav1.Object, error)

// registerKindGetters wires one informer + lister per watched kind and returns
// a lookup table keyed by the lowercased Kubernetes "kind" string (matching the
// value the k8seventsreceiver puts in the k8s.object.kind resource attribute).
//
// Calling .Lister() on a SharedInformerFactory implicitly registers the backing
// informer, so factory.Start() will start every watch referenced here.
//
// To support a new kind, add one entry below (and grant it in RBAC). For
// cluster-scoped kinds, ignore the namespace argument in the closure.
func registerKindGetters(f informers.SharedInformerFactory) map[string]objectGetter {
	pods := f.Core().V1().Pods().Lister()
	services := f.Core().V1().Services().Lister()
	deployments := f.Apps().V1().Deployments().Lister()
	replicaSets := f.Apps().V1().ReplicaSets().Lister()
	statefulSets := f.Apps().V1().StatefulSets().Lister()
	daemonSets := f.Apps().V1().DaemonSets().Lister()
	jobs := f.Batch().V1().Jobs().Lister()
	cronJobs := f.Batch().V1().CronJobs().Lister()

	return map[string]objectGetter{
		"pod":         func(ns, n string) (metav1.Object, error) { return pods.Pods(ns).Get(n) },
		"service":     func(ns, n string) (metav1.Object, error) { return services.Services(ns).Get(n) },
		"deployment":  func(ns, n string) (metav1.Object, error) { return deployments.Deployments(ns).Get(n) },
		"replicaset":  func(ns, n string) (metav1.Object, error) { return replicaSets.ReplicaSets(ns).Get(n) },
		"statefulset": func(ns, n string) (metav1.Object, error) { return statefulSets.StatefulSets(ns).Get(n) },
		"daemonset":   func(ns, n string) (metav1.Object, error) { return daemonSets.DaemonSets(ns).Get(n) },
		"job":         func(ns, n string) (metav1.Object, error) { return jobs.Jobs(ns).Get(n) },
		"cronjob":     func(ns, n string) (metav1.Object, error) { return cronJobs.CronJobs(ns).Get(n) },
	}
}
