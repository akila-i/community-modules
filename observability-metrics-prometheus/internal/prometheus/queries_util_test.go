// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import "testing"

func TestBuildSumByClause(t *testing.T) {
	tests := []struct {
		name        string
		metricLabel string
		scopeLabels []string
		want        string
	}{
		{"empty", "", nil, ""},
		{"metric only", "container", nil, "container"},
		{"scope only", "", []string{"label_a", "label_b"}, "label_a, label_b"},
		{"both", "container", []string{"label_a"}, "label_a, container"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSumByClause(tt.metricLabel, tt.scopeLabels)
			if got != tt.want {
				t.Errorf("BuildSumByClause() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildHistogramSumByClause(t *testing.T) {
	tests := []struct {
		name        string
		sumByClause string
		want        string
	}{
		{"empty", "", "le"},
		{"whitespace", "   ", "le"},
		{"with labels", "label_a, label_b", "label_a, label_b, le"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildHistogramSumByClause(tt.sumByClause)
			if got != tt.want {
				t.Errorf("BuildHistogramSumByClause() = %q, want %q", got, tt.want)
			}
		})
	}
}
