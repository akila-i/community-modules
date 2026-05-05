// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

// HTTPMetricsQuerier is implemented by any L7 metrics provider that surfaces
// HTTP observability data through Prometheus. Swap providers at wire-up time by
// injecting a different implementation — no handler code changes required.
type HTTPMetricsQuerier interface {
	// LabelFilter builds a Prometheus selector string that scopes a query to the
	// given namespace/component/project/environment identifiers.
	LabelFilter(namespace, componentUID, projectUID, environmentUID string) string

	// ScopeLabelNames returns the Prometheus label names that correspond to the
	// non-empty scope identifiers, used to build sum-by clauses.
	ScopeLabelNames(componentUID, projectUID, environmentUID string) []string

	RequestCountQuery(labelFilter, sumByClause string) string
	SuccessfulRequestCountQuery(labelFilter, sumByClause string) string
	UnsuccessfulRequestCountQuery(labelFilter, sumByClause string) string
	MeanLatencyQuery(labelFilter, sumByClause string) string
	P50LatencyQuery(labelFilter, sumByClause string) string
	P90LatencyQuery(labelFilter, sumByClause string) string
	P99LatencyQuery(labelFilter, sumByClause string) string
}
