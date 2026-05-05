// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import "strings"

// Kubernetes label constants shared across all metrics providers.
const (
	LabelComponentUID   = "openchoreo.dev/component-uid"
	LabelProjectUID     = "openchoreo.dev/project-uid"
	LabelEnvironmentUID = "openchoreo.dev/environment-uid"
	LabelNamespace      = "openchoreo.dev/namespace"
)

// prometheusLabelName converts a Kubernetes label name to the kube-state-metrics
// Prometheus label convention (character sanitization + "label_" prefix).
// e.g., "openchoreo.dev/component-uid" → "label_openchoreo_dev_component_uid"
func prometheusLabelName(kubernetesLabel string) string {
	label := strings.ReplaceAll(kubernetesLabel, "-", "_")
	label = strings.ReplaceAll(label, ".", "_")
	label = strings.ReplaceAll(label, "/", "_")
	return "label_" + label
}
