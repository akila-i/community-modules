// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import "testing"

func TestPrometheusLabelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openchoreo.dev/component-uid", "label_openchoreo_dev_component_uid"},
		{"openchoreo.dev/project-uid", "label_openchoreo_dev_project_uid"},
		{"openchoreo.dev/environment-uid", "label_openchoreo_dev_environment_uid"},
		{"openchoreo.dev/namespace", "label_openchoreo_dev_namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := prometheusLabelName(tt.input)
			if got != tt.expected {
				t.Errorf("prometheusLabelName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
