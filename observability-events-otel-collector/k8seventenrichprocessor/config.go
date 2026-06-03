// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventenrichprocessor // import "github.com/openchoreo/community-modules/observability-events-otel-collector/k8seventenrichprocessor"

import (
	"fmt"
	"time"
)

// Config defines the configuration for the k8seventenrich processor.
//
// For each Kubernetes event, the processor looks up the involved object in its
// informer cache and copies the requested metadata onto the event's resource
// attributes. Each metadata kind (labels, annotations, owner references) is
// controlled by its own block so they can be enabled/filtered independently.
type Config struct {
	// ResyncPeriod is how often the shared informer caches are fully re-listed
	// against the API server. Zero disables periodic resync and relies purely
	// on the watch stream. Defaults to 10m.
	ResyncPeriod time.Duration `mapstructure:"resync_period"`

	// CacheSyncTimeout bounds how long Start waits for the informer caches to
	// warm up before failing. Without a bound a missing RBAC grant would hang
	// startup forever; with one, startup fails loudly and the pod crash-loops
	// with a clear error. Raise it for large or slow clusters. Defaults to 2m.
	CacheSyncTimeout time.Duration `mapstructure:"cache_sync_timeout"`

	// Labels controls enrichment from the involved object's labels.
	Labels FieldConfig `mapstructure:"labels"`

	// Annotations controls enrichment from the involved object's annotations.
	Annotations FieldConfig `mapstructure:"annotations"`

	// OwnerReferences controls enrichment from the involved object's
	// controlling owner reference.
	OwnerReferences OwnerConfig `mapstructure:"owner_references"`
}

// FieldConfig controls enrichment from a key/value metadata map (labels or
// annotations).
type FieldConfig struct {
	// Enabled turns this metadata source on or off.
	Enabled bool `mapstructure:"enabled"`

	// Prefix is prepended to every key copied onto the resource attributes.
	Prefix string `mapstructure:"prefix"`

	// Include, when non-empty, restricts enrichment to this allow-list of keys.
	// Empty means copy all keys (minus Exclude).
	Include []string `mapstructure:"include"`

	// Exclude is a deny-list of keys that are never copied, applied even when
	// Include is empty. Note: setting this in config replaces any default
	// (it is not merged with it).
	Exclude []string `mapstructure:"exclude"`
}

// OwnerConfig controls enrichment from the involved object's controlling owner
// reference (the ownerReference with controller=true).
type OwnerConfig struct {
	// Enabled turns owner-reference enrichment on or off.
	Enabled bool `mapstructure:"enabled"`

	// Prefix is prepended to the emitted owner keys (kind, name, uid).
	Prefix string `mapstructure:"prefix"`
}

// Validate checks that the configuration is well formed.
func (cfg *Config) Validate() error {
	if cfg.ResyncPeriod < 0 {
		return fmt.Errorf("resync_period must not be negative, got %s", cfg.ResyncPeriod)
	}
	if cfg.CacheSyncTimeout <= 0 {
		return fmt.Errorf("cache_sync_timeout must be positive, got %s", cfg.CacheSyncTimeout)
	}
	if cfg.Labels.Enabled && cfg.Labels.Prefix == "" {
		return fmt.Errorf("labels.prefix must not be empty when labels are enabled")
	}
	if cfg.Annotations.Enabled && cfg.Annotations.Prefix == "" {
		return fmt.Errorf("annotations.prefix must not be empty when annotations are enabled")
	}
	if cfg.OwnerReferences.Enabled && cfg.OwnerReferences.Prefix == "" {
		return fmt.Errorf("owner_references.prefix must not be empty when owner_references are enabled")
	}
	return nil
}
