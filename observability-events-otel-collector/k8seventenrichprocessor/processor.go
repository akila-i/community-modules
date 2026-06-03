// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventenrichprocessor // import "github.com/openchoreo/community-modules/observability-events-otel-collector/k8seventenrichprocessor"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Attributes set by the k8seventsreceiver for the involved object.
const (
	attrObjectKind = "k8s.object.kind" // resource attribute
	attrObjectName = "k8s.object.name" // resource attribute
	// Namespace of the involved object lives on the LOG RECORD, not the resource.
	attrNamespaceName = "k8s.namespace.name" // record attribute
)

// defaultCacheSyncTimeout bounds the initial WaitForCacheSync in Start. Without
// a bound, a missing RBAC grant (a watched kind the service account may not
// list/watch) makes the informer retry forever and Start hangs silently. With
// a bound, startup fails loudly and the pod crash-loops with a clear error.
const defaultCacheSyncTimeout = 2 * time.Minute

// enrichProcessor enriches Kubernetes events with metadata of the object that
// triggered the event. It keeps cluster-wide informer caches of the watched
// workload kinds so enrichment is a local cache lookup with no per-event API call.
type enrichProcessor struct {
	logger *zap.Logger
	next   consumer.Logs

	labels       fieldExtractor
	annotations  fieldExtractor
	owners       ownerExtractor
	resyncPeriod time.Duration

	// cacheSyncTimeout bounds the initial cache warm-up in Start.
	cacheSyncTimeout time.Duration

	// newClientset is the source of the Kubernetes client; overridable in tests.
	newClientset func() (kubernetes.Interface, error)

	// Established in Start.
	factory  informers.SharedInformerFactory
	getters  map[string]objectGetter
	stopCh   chan struct{}
	stopOnce sync.Once
}

func newProcessor(set processor.Settings, cfg *Config, next consumer.Logs) *enrichProcessor {
	return &enrichProcessor{
		logger:           set.Logger,
		next:             next,
		labels:           newFieldExtractor(cfg.Labels),
		annotations:      newFieldExtractor(cfg.Annotations),
		owners:           newOwnerExtractor(cfg.OwnerReferences),
		resyncPeriod:     cfg.ResyncPeriod,
		cacheSyncTimeout: cfg.CacheSyncTimeout,
		newClientset:     inClusterClientset,
	}
}

// enrichmentEnabled reports whether any metadata source is active. When nothing
// is enabled, ConsumeLogs skips the cache lookup entirely.
func (ep *enrichProcessor) enrichmentEnabled() bool {
	return ep.labels.enabled || ep.annotations.enabled || ep.owners.enabled
}

// inClusterClientset builds a Kubernetes clientset from the pod's mounted
// service account credentials.
func inClusterClientset() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}

// Start builds the clientset, registers the per-kind informers, starts the
// watches, and blocks until the caches are warm. Enrichment only begins once
// the caches have synced.
func (ep *enrichProcessor) Start(ctx context.Context, _ component.Host) error {
	// With every metadata source disabled the processor is a pass-through
	// (ConsumeLogs short-circuits), so there is no reason to build clientset,
	// start cluster-wide watches, or require workload RBAC.
	if !ep.enrichmentEnabled() {
		ep.logger.Info("k8seventenrich: all enrichment sources disabled; running as pass-through (no caches started)")
		return nil
	}

	clientset, err := ep.newClientset()
	if err != nil {
		return fmt.Errorf("k8seventenrich: %w", err)
	}

	ep.stopCh = make(chan struct{})
	ep.factory = informers.NewSharedInformerFactory(clientset, ep.resyncPeriod)
	ep.getters = registerKindGetters(ep.factory)

	ep.factory.Start(ep.stopCh)
	if err := ep.waitForCacheSync(ctx); err != nil {
		ep.stopInformers() // don't leak the watches we just started
		return err
	}

	ep.logger.Info("k8seventenrich caches synced; enrichment active",
		zap.Int("watched_kinds", len(ep.getters)),
		zap.Bool("labels", ep.labels.enabled),
		zap.Bool("annotations", ep.annotations.enabled),
		zap.Bool("owner_references", ep.owners.enabled),
	)
	return nil
}

// waitForCacheSync blocks until every registered informer cache has synced.
// The wait ends early — and returns an error — if the start context is
// cancelled, the processor is shut down, or cacheSyncTimeout elapses. The
// timeout turns a silent hang (typically a missing RBAC grant) into a clear,
// crash-looping startup failure.
func (ep *enrichProcessor) waitForCacheSync(ctx context.Context) error {
	syncCtx, cancel := context.WithTimeout(ctx, ep.cacheSyncTimeout)
	defer cancel()

	// Bridge syncCtx (and our stop channel) onto the stop channel that
	// WaitForCacheSync understands. The goroutine is released either when the
	// context fires or, on the success path, by the deferred cancel above — so
	// it never outlives this call.
	stopCh := ep.stopCh
	syncStopCh := make(chan struct{})
	go func() {
		select {
		case <-syncCtx.Done():
		case <-stopCh:
		}
		close(syncStopCh)
	}()

	for typ, ok := range ep.factory.WaitForCacheSync(syncStopCh) {
		if !ok {
			return fmt.Errorf("k8seventenrich: informer cache for %s did not sync within %s; "+
				"check the service account has get/list/watch on every watched kind", typ, ep.cacheSyncTimeout)
		}
	}
	return nil
}

// Shutdown stops the informer watches.
func (ep *enrichProcessor) Shutdown(_ context.Context) error {
	ep.stopInformers()
	return nil
}

// stopInformers signals the informer watches to stop and waits for their
// goroutines to drain. It runs at most once (sync.Once), so Start's error path
// and Shutdown can both call it without risking a double close.
func (ep *enrichProcessor) stopInformers() {
	ep.stopOnce.Do(func() {
		if ep.stopCh != nil {
			close(ep.stopCh)
		}
		if ep.factory != nil {
			ep.factory.Shutdown() // blocks until informer goroutines exit
		}
	})
}

// Capabilities reports that this processor mutates the data it sees.
func (ep *enrichProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeLogs enriches each event's resource attributes with the involved
// object's metadata, then forwards the batch. Enrichment is best-effort: any
// event whose kind is unwatched, or whose object is not in cache, passes
// through unchanged.
func (ep *enrichProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if !ep.enrichmentEnabled() {
		return ep.next.ConsumeLogs(ctx, ld)
	}

	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		resAttrs := rl.Resource().Attributes()

		kind, ok := getStr(resAttrs, attrObjectKind)
		if !ok || kind == "" {
			continue
		}
		name, ok := getStr(resAttrs, attrObjectName)
		if !ok || name == "" {
			continue
		}
		get, watched := ep.getters[strings.ToLower(kind)]
		if !watched {
			continue // kind we don't watch (e.g. Node, Namespace, a CRD)
		}

		// Namespace lives on the log record, not the resource.
		namespace := namespaceFromRecords(rl)
		if namespace == "" {
			continue // cluster-scoped or missing; namespaced listers need it
		}

		obj, err := get(namespace, name)
		if err != nil {
			// Not found / not yet synced. Leave the event unchanged.
			continue
		}

		ep.labels.inject(resAttrs, obj.GetLabels())
		ep.annotations.inject(resAttrs, obj.GetAnnotations())
		ep.owners.inject(resAttrs, obj.GetOwnerReferences())
	}

	return ep.next.ConsumeLogs(ctx, ld)
}

// namespaceFromRecords returns the first non-empty k8s.namespace.name found on
// any log record under the resource. For events emitted by k8seventsreceiver
// there is exactly one record per resource, all sharing the involved object.
func namespaceFromRecords(rl plog.ResourceLogs) string {
	scopeLogs := rl.ScopeLogs()
	for i := 0; i < scopeLogs.Len(); i++ {
		records := scopeLogs.At(i).LogRecords()
		for j := 0; j < records.Len(); j++ {
			if v, ok := records.At(j).Attributes().Get(attrNamespaceName); ok {
				if s := v.Str(); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func getStr(m pcommon.Map, key string) (string, bool) {
	v, ok := m.Get(key)
	if !ok {
		return "", false
	}
	return v.AsString(), true
}
