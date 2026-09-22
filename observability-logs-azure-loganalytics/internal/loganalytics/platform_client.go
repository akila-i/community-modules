// Copyright 2026 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package loganalytics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs"
)

// GetPlatformLogs runs a platform-logs query and returns the page of records
// together with the total number matching, capped at MaxPlatformTotal.
func (c *Client) GetPlatformLogs(ctx context.Context, p PlatformLogsParams) (*PlatformLogsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	startedAt := time.Now()

	resp, err := c.api.QueryWorkspace(ctx, c.workspaceID, azlogs.QueryBody{
		Query:    to.Ptr(BuildPlatformLogsKQL(p)),
		Timespan: to.Ptr(azlogs.NewTimeInterval(p.StartTime, p.EndTime)),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("loganalytics: GetPlatformLogs: %w", err)
	}

	records, ok := findTable(resp.Tables, "LogMessage")
	if !ok {
		return &PlatformLogsResult{
			Logs:   []PlatformLogEntry{},
			TookMs: int(time.Since(startedAt).Milliseconds()),
		}, nil
	}

	entries, skipped, err := mapPlatformRows(records)
	if err != nil {
		return nil, err
	}
	for _, s := range skipped {
		c.logger.Warn("skipping malformed platform log record", slog.Any("error", s))
	}

	total, ok := findScalarTable(resp.Tables, "Total")
	if !ok {
		// The count statement produced nothing to read. The page is still
		// correct, so serve it rather than failing, and say what was lost.
		c.logger.Warn("platform logs response carried no Total column; reporting the page size instead")
		total = int64(len(entries))
	}
	if total > MaxPlatformTotal {
		total = MaxPlatformTotal
	}

	return &PlatformLogsResult{
		Logs:       entries,
		TotalCount: int(total),
		TookMs:     int(time.Since(startedAt).Milliseconds()),
	}, nil
}

// GetPlatformLogFilterValues lists the distinct values one filter takes under
// a query, with a count for each.
func (c *Client) GetPlatformLogFilterValues(
	ctx context.Context, p PlatformLogFilterValuesParams,
) (*PlatformLogFilterValuesResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	startedAt := time.Now()

	resp, err := c.api.QueryWorkspace(ctx, c.workspaceID, azlogs.QueryBody{
		Query:    to.Ptr(BuildPlatformLogFilterValuesKQL(p)),
		Timespan: to.Ptr(azlogs.NewTimeInterval(p.Query.StartTime, p.Query.EndTime)),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("loganalytics: GetPlatformLogFilterValues: %w", err)
	}

	values := []PlatformLogFilterValue{}
	if t, ok := findTable(resp.Tables, "Value"); ok {
		values, err = mapPlatformFilterValues(t)
		if err != nil {
			return nil, err
		}
	}

	total, ok := findScalarTable(resp.Tables, "TotalValues")
	if !ok {
		total = int64(len(values))
	}

	return &PlatformLogFilterValuesResult{
		Filter:      p.Filter,
		Values:      values,
		TotalValues: total,
		TookMs:      int(time.Since(startedAt).Milliseconds()),
	}, nil
}
