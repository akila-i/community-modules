// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package opensearch

import (
	"sort"
	"strings"
	"time"
)

// ClusterInstanceField is the record-level cluster identifier the logs collector stamps.
// It is a flat key - no dots, no slashes - so it survives Fluent Bit's Replace_Dots and
// each backend's own field-name normalisation unchanged.
const ClusterInstanceField = "openchoreo_cluster_instance"

// ClusterLogsQueryParams holds parameters for a cluster log query.
// Every filter is optional except the time range. Multi-value fields OR within
// themselves and AND with each other, and an empty field is not a filter.
type ClusterLogsQueryParams struct {
	ClusterInstances []string
	Namespaces       []string
	PodNames         []string
	ContainerNames   []string
	Labels           map[string]string
	StartTime        string
	EndTime          string
	SearchPhrase     string
	LogLevels        []string
	Limit            int
	SortOrder        string
}

// BuildClusterLogsQuery builds a query over raw Kubernetes coordinates, with no
// project/component/environment correlation.
func (qb *QueryBuilder) BuildClusterLogsQuery(params ClusterLogsQueryParams) map[string]interface{} {
	mustConditions := []map[string]interface{}{}
	mustConditions = addTimeRangeFilter(mustConditions, params.StartTime, params.EndTime)
	mustConditions = addTermsFilter(mustConditions, ClusterInstanceField, params.ClusterInstances)
	mustConditions = addTermsFilter(mustConditions, KubernetesNamespaceName, params.Namespaces)
	mustConditions = addTermsFilter(mustConditions, KubernetesPodName, params.PodNames)
	mustConditions = addTermsFilter(mustConditions, KubernetesContainerName, params.ContainerNames)
	mustConditions = addLabelFilters(mustConditions, params.Labels)
	mustConditions = addSearchPhraseFilter(mustConditions, params.SearchPhrase)
	mustConditions = addLogLevelFilter(mustConditions, params.LogLevels)

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	sortOrder := params.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	return map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": mustConditions,
			},
		},
		"sort": []map[string]interface{}{
			{
				"@timestamp": map[string]interface{}{
					"order": sortOrder,
				},
			},
		},
	}
}

// addTermsFilter ORs the given values on one field. An empty slice is not a filter.
func addTermsFilter(
	mustConditions []map[string]interface{}, field string, values []string,
) []map[string]interface{} {
	if len(values) == 0 {
		return mustConditions
	}
	return append(mustConditions, map[string]interface{}{
		"terms": map[string]interface{}{
			field: values,
		},
	})
}

// addLabelFilters ANDs one term per label pair.
//
// Label keys are rewritten with ReplaceDots to match how Fluent Bit stores them: the
// OpenSearch output runs with Replace_Dots On, so a pod labelled openchoreo.dev/plane is
// indexed at kubernetes.labels.openchoreo_dev/plane. Callers pass the label key as
// Kubernetes spells it and this is the only place that has to know.
//
// Keys are sorted so the query is deterministic, which keeps it comparable in tests and
// stable in logs.
func addLabelFilters(
	mustConditions []map[string]interface{}, labels map[string]string,
) []map[string]interface{} {
	if len(labels) == 0 {
		return mustConditions
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		mustConditions = append(mustConditions, map[string]interface{}{
			"term": map[string]interface{}{
				KubernetesLabelsPrefix + "." + ReplaceDots(k): labels[k],
			},
		})
	}
	return mustConditions
}

// ClusterLogEntry is a parsed cluster log entry: the message plus the physical
// coordinates and pod metadata of whatever produced it.
type ClusterLogEntry struct {
	Timestamp       time.Time         `json:"timestamp"`
	Log             string            `json:"log"`
	LogLevel        string            `json:"logLevel"`
	ClusterInstance string            `json:"clusterInstance"`
	NamespaceName   string            `json:"namespaceName"`
	PodName         string            `json:"podName"`
	ContainerName   string            `json:"containerName"`
	PodIP           string            `json:"podIp"`
	NodeName        string            `json:"nodeName"`
	ContainerImage  string            `json:"containerImage"`
	Labels          map[string]string `json:"labels"`
}

// ParseClusterLogEntry converts a search hit to a ClusterLogEntry.
func ParseClusterLogEntry(hit Hit) ClusterLogEntry {
	source := hit.Source
	entry := ClusterLogEntry{
		ClusterInstance: getStringValue(source, ClusterInstanceField),
	}

	if ts, ok := source["@timestamp"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = parsed
		}
	}
	if log, ok := source["log"].(string); ok {
		entry.Log = log
		entry.LogLevel = extractLogLevel(log)
	}
	if k8s, ok := source["kubernetes"].(map[string]interface{}); ok {
		entry.NamespaceName = getStringValue(k8s, "namespace_name")
		entry.PodName = getStringValue(k8s, "pod_name")
		entry.ContainerName = getStringValue(k8s, "container_name")
		entry.PodIP = getStringValue(k8s, "pod_ip")
		// Fluent Bit's kubernetes filter calls the node "host".
		entry.NodeName = getStringValue(k8s, "host")
		entry.ContainerImage = getStringValue(k8s, "container_image")

		if labels, ok := k8s["labels"].(map[string]interface{}); ok {
			entry.Labels = make(map[string]string, len(labels))
			for k, v := range labels {
				if str, ok := v.(string); ok {
					entry.Labels[RestoreLabelKey(k)] = str
				}
			}
		}
	}
	return entry
}

// RestoreLabelKey undoes Fluent Bit's Replace_Dots on a label key, so a caller sees the
// key as Kubernetes spells it and can paste it straight back into the label filter.
//
// Only the prefix - everything before the first "/" - is restored. A Kubernetes label
// prefix is a DNS subdomain and cannot contain an underscore, so every "_" there was a
// "."; the name part after the "/" may legitimately contain underscores (the workload
// labels "version_id" and "build-name" among them) and is left alone. Reversing the
// whole key would corrupt those.
//
// A prefix-less key that genuinely contained dots is not recoverable, since it is
// indistinguishable from one that contained underscores. Those are vanishingly rare and
// the alternative corrupts common keys, so it is left as stored.
func RestoreLabelKey(key string) string {
	slash := strings.Index(key, "/")
	if slash < 0 {
		return key
	}
	return strings.ReplaceAll(key[:slash], "_", ".") + key[slash:]
}
