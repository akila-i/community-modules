// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"strings"
	"testing"
)

func TestHubbleLabelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openchoreo.dev/component-uid", "openchoreo_dev_component_uid"},
		{"openchoreo.dev/project-uid", "openchoreo_dev_project_uid"},
		{"openchoreo.dev/environment-uid", "openchoreo_dev_environment_uid"},
		{"openchoreo.dev/namespace", "openchoreo_dev_namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := hubbleLabelName(tt.input)
			if got != tt.expected {
				t.Errorf("hubbleLabelName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHubbleQuerier_LabelFilter(t *testing.T) {
	q := HubbleQuerier{}

	tests := []struct {
		name           string
		namespace      string
		componentUID   string
		projectUID     string
		environmentUID string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "namespace only",
			namespace:    "test-ns",
			wantContains: []string{`openchoreo_dev_namespace="test-ns"`},
			wantNotContain: []string{
				"label_",
				"openchoreo_dev_component_uid",
				"openchoreo_dev_project_uid",
				"openchoreo_dev_environment_uid",
			},
		},
		{
			name:           "all fields",
			namespace:      "test-ns",
			componentUID:   "c5f0a8d3-7e2b-4d9c-a1f4-6b8e3c0d5a7f",
			projectUID:     "d6a1b9e4-8f3c-4e0d-b2a5-7c9f4d1e6b8a",
			environmentUID: "e7b2c0f5-9a4d-4f1e-c3b6-8d0a5e2f7c9b",
			wantContains: []string{
				`openchoreo_dev_namespace="test-ns"`,
				`openchoreo_dev_component_uid="c5f0a8d3-7e2b-4d9c-a1f4-6b8e3c0d5a7f"`,
				`openchoreo_dev_project_uid="d6a1b9e4-8f3c-4e0d-b2a5-7c9f4d1e6b8a"`,
				`openchoreo_dev_environment_uid="e7b2c0f5-9a4d-4f1e-c3b6-8d0a5e2f7c9b"`,
			},
			wantNotContain: []string{"label_"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := q.LabelFilter(tt.namespace, tt.componentUID, tt.projectUID, tt.environmentUID)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("LabelFilter() = %q, want to contain %q", got, want)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("LabelFilter() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}

func TestHubbleQuerier_ScopeLabelNames(t *testing.T) {
	q := HubbleQuerier{}

	tests := []struct {
		name           string
		componentUID   string
		projectUID     string
		environmentUID string
		wantLen        int
		wantLabels     []string
	}{
		{"none", "", "", "", 0, nil},
		{"component only", "comp-1", "", "", 1, []string{"openchoreo_dev_component_uid"}},
		{"all", "comp-1", "proj-1", "env-1", 3, []string{
			"openchoreo_dev_component_uid",
			"openchoreo_dev_project_uid",
			"openchoreo_dev_environment_uid",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := q.ScopeLabelNames(tt.componentUID, tt.projectUID, tt.environmentUID)
			if len(got) != tt.wantLen {
				t.Errorf("ScopeLabelNames() returned %d labels, want %d: %v", len(got), tt.wantLen, got)
			}
			for _, want := range tt.wantLabels {
				found := false
				for _, label := range got {
					if label == want {
						found = true
					}
					if strings.HasPrefix(label, "label_") {
						t.Errorf("ScopeLabelNames() label %q must not have label_ prefix", label)
					}
				}
				if !found {
					t.Errorf("ScopeLabelNames() = %v, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestHubbleQuerier_QueryMethods(t *testing.T) {
	q := HubbleQuerier{}
	labelFilter := `openchoreo_dev_namespace="test-ns",openchoreo_dev_component_uid="abc"`
	sumByClause := "openchoreo_dev_component_uid"

	tests := []struct {
		name        string
		queryFn     func(string, string) string
		contains    []string
		notContains []string
	}{
		{
			"RequestCount",
			q.RequestCountQuery,
			[]string{"hubble_http_requests_total", `reporter="server"`, labelFilter, sumByClause},
			[]string{"kube_pod_labels", "label_replace", "destination_workload"},
		},
		{
			"SuccessfulRequestCount",
			q.SuccessfulRequestCountQuery,
			[]string{"hubble_http_requests_total", `reporter="server"`, `status=~"^[123]..?$"`, labelFilter},
			[]string{"kube_pod_labels", "label_replace"},
		},
		{
			"UnsuccessfulRequestCount",
			q.UnsuccessfulRequestCountQuery,
			[]string{"hubble_http_requests_total", `reporter="server"`, `status=~"^[45]..?$"`, labelFilter},
			[]string{"kube_pod_labels", "label_replace"},
		},
		{
			"MeanLatency",
			q.MeanLatencyQuery,
			[]string{"hubble_http_request_duration_seconds_sum", `reporter="server"`, labelFilter},
			[]string{"kube_pod_labels", "label_replace"},
		},
		{
			"P50Latency",
			q.P50LatencyQuery,
			[]string{"histogram_quantile", "0.5", `reporter="server"`, labelFilter},
			[]string{"kube_pod_labels", "label_replace"},
		},
		{
			"P90Latency",
			q.P90LatencyQuery,
			[]string{"histogram_quantile", "0.9", `reporter="server"`, labelFilter},
			[]string{"kube_pod_labels", "label_replace"},
		},
		{
			"P99Latency",
			q.P99LatencyQuery,
			[]string{"histogram_quantile", "0.99", `reporter="server"`, labelFilter},
			[]string{"kube_pod_labels", "label_replace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.queryFn(labelFilter, sumByClause)
			for _, want := range tt.contains {
				if !strings.Contains(query, want) {
					t.Errorf("%s: query = %q, want to contain %q", tt.name, query, want)
				}
			}
			for _, notWant := range tt.notContains {
				if strings.Contains(query, notWant) {
					t.Errorf("%s: query = %q, should not contain %q", tt.name, query, notWant)
				}
			}
		})
	}
}

// TestHubbleQuerierImplementsInterface verifies the interface is satisfied at compile time.
var _ HTTPMetricsQuerier = HubbleQuerier{}
