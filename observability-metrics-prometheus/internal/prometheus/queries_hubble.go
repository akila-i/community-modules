// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"fmt"
	"strings"
)

// HubbleQuerier implements HTTPMetricsQuerier using Cilium Hubble as the L7
// metrics source. Hubble embeds OpenChoreo pod labels directly into metric
// series (no kube_pod_labels join required).
type HubbleQuerier struct{}

// hubbleLabelName converts a Kubernetes label name to the Prometheus label name
// that Hubble produces when embedding pod labels into metrics. Hubble sanitizes
// characters but does not add a "label_" prefix.
// e.g., "openchoreo.dev/component-uid" → "openchoreo_dev_component_uid"
func hubbleLabelName(kubernetesLabel string) string {
	label := strings.ReplaceAll(kubernetesLabel, "-", "_")
	label = strings.ReplaceAll(label, ".", "_")
	label = strings.ReplaceAll(label, "/", "_")
	return label
}

func (HubbleQuerier) LabelFilter(namespace, componentUID, projectUID, environmentUID string) string {
	filter := fmt.Sprintf("%s=%q", hubbleLabelName(LabelNamespace), namespace)
	if componentUID != "" {
		filter = fmt.Sprintf("%s,%s=%q", filter, hubbleLabelName(LabelComponentUID), componentUID)
	}
	if projectUID != "" {
		filter = fmt.Sprintf("%s,%s=%q", filter, hubbleLabelName(LabelProjectUID), projectUID)
	}
	if environmentUID != "" {
		filter = fmt.Sprintf("%s,%s=%q", filter, hubbleLabelName(LabelEnvironmentUID), environmentUID)
	}
	return filter
}

func (HubbleQuerier) ScopeLabelNames(componentUID, projectUID, environmentUID string) []string {
	labels := make([]string, 0, 3)
	if componentUID != "" {
		labels = append(labels, hubbleLabelName(LabelComponentUID))
	}
	if projectUID != "" {
		labels = append(labels, hubbleLabelName(LabelProjectUID))
	}
	if environmentUID != "" {
		labels = append(labels, hubbleLabelName(LabelEnvironmentUID))
	}
	return labels
}

func (HubbleQuerier) RequestCountQuery(labelFilter, sumByClause string) string {
	return fmt.Sprintf(`
    sum by (%s) (
        rate(hubble_http_requests_total{reporter="server",%s}[2m])
    )
    >= 0
`, sumByClause, labelFilter)
}

func (HubbleQuerier) SuccessfulRequestCountQuery(labelFilter, sumByClause string) string {
	return fmt.Sprintf(`
    sum by (%s) (
        rate(hubble_http_requests_total{reporter="server",status=~"^[123]..?$",%s}[2m])
    )
    >= 0
`, sumByClause, labelFilter)
}

func (HubbleQuerier) UnsuccessfulRequestCountQuery(labelFilter, sumByClause string) string {
	return fmt.Sprintf(`
    sum by (%s) (
        rate(hubble_http_requests_total{reporter="server",status=~"^[45]..?$",%s}[2m])
    )
    >= 0
`, sumByClause, labelFilter)
}

func (HubbleQuerier) MeanLatencyQuery(labelFilter, sumByClause string) string {
	return fmt.Sprintf(`
    (
        sum by (%s) (
            rate(hubble_http_request_duration_seconds_sum{reporter="server",%s}[2m])
        )
        /
        sum by (%s) (
            rate(hubble_http_requests_total{reporter="server",%s}[2m])
        )
    )
    >= 0
`, sumByClause, labelFilter, sumByClause, labelFilter)
}

func (HubbleQuerier) P50LatencyQuery(labelFilter, sumByClause string) string {
	histogramSumByClause := BuildHistogramSumByClause(sumByClause)
	return fmt.Sprintf(`
    histogram_quantile(
        0.5,
        sum by (%s) (
            rate(hubble_http_request_duration_seconds_bucket{reporter="server",%s}[2m])
        )
    )
    >= 0
`, histogramSumByClause, labelFilter)
}

func (HubbleQuerier) P90LatencyQuery(labelFilter, sumByClause string) string {
	histogramSumByClause := BuildHistogramSumByClause(sumByClause)
	return fmt.Sprintf(`
    histogram_quantile(
        0.9,
        sum by (%s) (
            rate(hubble_http_request_duration_seconds_bucket{reporter="server",%s}[2m])
        )
    )
    >= 0
`, histogramSumByClause, labelFilter)
}

func (HubbleQuerier) P99LatencyQuery(labelFilter, sumByClause string) string {
	histogramSumByClause := BuildHistogramSumByClause(sumByClause)
	return fmt.Sprintf(`
    histogram_quantile(
        0.99,
        sum by (%s) (
            rate(hubble_http_request_duration_seconds_bucket{reporter="server",%s}[2m])
        )
    )
    >= 0
`, histogramSumByClause, labelFilter)
}
