// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"fmt"
	"strings"
)

// BuildSumByClause builds the label list for a PromQL "sum by (...)" clause.
func BuildSumByClause(metricLabel string, scopeLabels []string) string {
	sumByLabels := make([]string, 0, len(scopeLabels)+1)
	sumByLabels = append(sumByLabels, scopeLabels...)
	if metricLabel != "" {
		sumByLabels = append(sumByLabels, metricLabel)
	}
	return strings.Join(sumByLabels, ", ")
}

// BuildHistogramSumByClause appends the required "le" label to a sum-by clause
// for use in histogram_quantile expressions.
func BuildHistogramSumByClause(sumByClause string) string {
	if strings.TrimSpace(sumByClause) == "" {
		return "le"
	}
	return fmt.Sprintf("%s, le", sumByClause)
}
