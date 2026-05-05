// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"fmt"
	"strings"
)

// BuildLabelFilter builds a kube-state-metrics label selector string for filtering
// kube_pod_labels. The namespace is always included; other UIDs are omitted when empty.
func BuildLabelFilter(namespace, componentUID, projectUID, environmentUID string) string {
	namespaceLabel := prometheusLabelName(LabelNamespace)
	componentLabel := prometheusLabelName(LabelComponentUID)
	projectLabel := prometheusLabelName(LabelProjectUID)
	environmentLabel := prometheusLabelName(LabelEnvironmentUID)

	filter := fmt.Sprintf("%s=%q", namespaceLabel, namespace)
	if componentUID != "" {
		filter = fmt.Sprintf("%s,%s=%q", filter, componentLabel, componentUID)
	}
	if projectUID != "" {
		filter = fmt.Sprintf("%s,%s=%q", filter, projectLabel, projectUID)
	}
	if environmentUID != "" {
		filter = fmt.Sprintf("%s,%s=%q", filter, environmentLabel, environmentUID)
	}
	return filter
}

// BuildScopeLabelNames returns the kube-state-metrics Prometheus label names for
// whichever of componentUID, projectUID, and environmentUID are non-empty.
func BuildScopeLabelNames(componentUID, projectUID, environmentUID string) []string {
	labels := make([]string, 0, 3)
	if componentUID != "" {
		labels = append(labels, prometheusLabelName(LabelComponentUID))
	}
	if projectUID != "" {
		labels = append(labels, prometheusLabelName(LabelProjectUID))
	}
	if environmentUID != "" {
		labels = append(labels, prometheusLabelName(LabelEnvironmentUID))
	}
	return labels
}

// BuildGroupLeftClause builds a PromQL group_left clause that propagates the given
// scope labels from the right-hand side of a vector match.
func BuildGroupLeftClause(scopeLabels []string) string {
	if len(scopeLabels) == 0 {
		return "group_left"
	}
	return fmt.Sprintf("group_left (%s)", strings.Join(scopeLabels, ", "))
}

// BuildComponentLabelFilter builds a kube-state-metrics label selector for
// component/project/environment UIDs without namespace. Used in alert rule expressions.
func BuildComponentLabelFilter(componentUID, projectUID, environmentUID string) string {
	return fmt.Sprintf(`%s=%q,%s=%q,%s=%q`,
		prometheusLabelName(LabelComponentUID), componentUID,
		prometheusLabelName(LabelProjectUID), projectUID,
		prometheusLabelName(LabelEnvironmentUID), environmentUID,
	)
}

// BuildCPUUsageQuery builds a PromQL query for CPU usage rate.
func BuildCPUUsageQuery(labelFilter, sumByClause, groupLeftClause string) string {
	return fmt.Sprintf(`sum by (%s) (
    rate(container_cpu_usage_seconds_total{container!=""}[2m]) * on (pod, namespace) %s kube_pod_labels{%s} )`, sumByClause, groupLeftClause, labelFilter)
}

// BuildCPURequestsQuery builds a PromQL query for CPU requests.
func BuildCPURequestsQuery(labelFilter, sumByClause, groupLeftClause string) string {
	return fmt.Sprintf(`sum by (%s) (
            (
                kube_pod_container_resource_requests{resource="cpu"}
                AND ON (pod, namespace)
                (kube_pod_status_phase{phase="Running"} == 1)
            )
          * ON (pod, namespace) %s
            kube_pod_labels{%s}
        )`, sumByClause, groupLeftClause, labelFilter)
}

// BuildCPULimitsQuery builds a PromQL query for CPU limits.
func BuildCPULimitsQuery(labelFilter, sumByClause, groupLeftClause string) string {
	return fmt.Sprintf(`sum by (%s) (
            (
                kube_pod_container_resource_limits{resource="cpu"}
                AND ON (pod, namespace)
                (kube_pod_status_phase{phase="Running"} == 1)
            )
          * ON (pod, namespace) %s
            kube_pod_labels{%s}
        )`, sumByClause, groupLeftClause, labelFilter)
}

// BuildMemoryUsageQuery builds a PromQL query for memory usage.
func BuildMemoryUsageQuery(labelFilter, sumByClause, groupLeftClause string) string {
	return fmt.Sprintf(`sum by (%s) (
              container_memory_working_set_bytes{container!=""}
              * on (pod, namespace) %s
                kube_pod_labels{%s}
            )`, sumByClause, groupLeftClause, labelFilter)
}

// BuildMemoryRequestsQuery builds a PromQL query for memory requests.
func BuildMemoryRequestsQuery(labelFilter, sumByClause, groupLeftClause string) string {
	return fmt.Sprintf(`sum by (%s) (
            (
                kube_pod_container_resource_requests{resource="memory"}
                AND ON (pod, namespace)
                (kube_pod_status_phase{phase="Running"} == 1)
            )
          * ON (pod, namespace) %s
            kube_pod_labels{%s}
        )`, sumByClause, groupLeftClause, labelFilter)
}

// BuildMemoryLimitsQuery builds a PromQL query for memory limits.
func BuildMemoryLimitsQuery(labelFilter, sumByClause, groupLeftClause string) string {
	return fmt.Sprintf(`sum by (%s) (
            (
                kube_pod_container_resource_limits{resource="memory"}
                AND ON (pod, namespace)
                (kube_pod_status_phase{phase="Running"} == 1)
            )
          * ON (pod, namespace) %s
            kube_pod_labels{%s}
        )`, sumByClause, groupLeftClause, labelFilter)
}
