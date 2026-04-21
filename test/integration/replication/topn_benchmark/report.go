// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with this work for
// additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package topn_benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// LatencyStats holds latency percentiles in milliseconds.
type LatencyStats struct {
	P50 float64
	P95 float64
	P99 float64
	Min float64
	Max float64
	Avg float64
}

// ComputeLatencyStats calculates latency statistics from a slice of durations.
func ComputeLatencyStats(durations []time.Duration) LatencyStats {
	if len(durations) == 0 {
		return LatencyStats{}
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	slices.Sort(sorted)

	toMS := func(d time.Duration) float64 {
		return float64(d.Nanoseconds()) / 1e6
	}

	var total time.Duration
	for _, d := range sorted {
		total += d
	}

	return LatencyStats{
		Min: toMS(sorted[0]),
		Max: toMS(sorted[len(sorted)-1]),
		Avg: toMS(total) / float64(len(sorted)),
		P50: toMS(sorted[len(sorted)*50/100]),
		P95: toMS(sorted[len(sorted)*95/100]),
		P99: toMS(sorted[len(sorted)*99/100]),
	}
}

// BenchmarkResult holds the result for a single benchmark scenario.
type BenchmarkResult struct {
	Scenario       string
	AggFunc        string
	TopN           int32
	Sort           string
	Latency        LatencyStats
	UniqueEntities int
	TotalItems     int
	EntityDetails  string
}

// BenchmarkConfig describes the test environment configuration.
type BenchmarkConfig struct {
	DataNodeCount    int
	ShardNum         int
	ServiceCount     int
	InstancePerSvc   int
	TimeBucketCount  int
	TimeBucketWidth   string
	DominantEntity   string
	TopNRuleName     string
	SourceMeasure    string
	Group            string
	EntityTags       []string
	ShardingKey      []string
	LRUSize          int
	CountersNumber   int
	QueryIterations  int
	SegmentInterval  string
	TTL              string
}

// PrintReport prints a Markdown-formatted benchmark report to stdout.
func PrintReport(results []BenchmarkResult, cfg BenchmarkConfig) {
	fmt.Println("\n## TopN Benchmark Report")
	printConfigSection(cfg)
	printResultTable(results)
	printLatencyTable(results)
}

func printConfigSection(cfg BenchmarkConfig) {
	fmt.Println("\n### Test Environment")
	fmt.Println("| Item | Value |")
	fmt.Println("|------|-------|")
	fmt.Printf("| Deployment Mode | Distributed (1 liaison + %d data nodes) |\n", cfg.DataNodeCount)

	fmt.Println("\n### Group Configuration")
	fmt.Println("| Item | Value |")
	fmt.Println("|------|-------|")
	fmt.Printf("| Group | %s |\n", cfg.Group)
	fmt.Printf("| Catalog | CATALOG_MEASURE |\n")
	fmt.Printf("| Shard Num | %d |\n", cfg.ShardNum)
	fmt.Printf("| Segment Interval | %s |\n", cfg.SegmentInterval)
	fmt.Printf("| TTL | %s |\n", cfg.TTL)

	fmt.Println("\n### Measure Schema")
	fmt.Println("| Item | Value |")
	fmt.Println("|------|-------|")
	fmt.Printf("| Measure | %s |\n", cfg.SourceMeasure)
	fmt.Printf("| Group | %s |\n", cfg.Group)
	fmt.Printf("| Entity Tags | %s |\n", strings.Join(cfg.EntityTags, ", "))
	fmt.Printf("| Sharding Key | %s |\n", strings.Join(cfg.ShardingKey, ", "))
	fmt.Printf("| Interval | %s |\n", cfg.TimeBucketWidth)
	fmt.Println("| Tags | instance_id (string), service_id (string) |")
	fmt.Println("| Fields | value (INT, Gorilla, ZSTD) |")

	fmt.Println("\n### TopNAggregation")
	fmt.Println("| Item | Value |")
	fmt.Println("|------|-------|")
	fmt.Printf("| TopN Rule | %s |\n", cfg.TopNRuleName)
	fmt.Printf("| Source Measure | %s |\n", cfg.SourceMeasure)
	fmt.Printf("| Field | value |\n")
	fmt.Printf("| Group By Tags | %s |\n", strings.Join(cfg.EntityTags, ", "))
	fmt.Printf("| Counters Number | %d |\n", cfg.CountersNumber)
	fmt.Printf("| LRU Size | %d |\n", cfg.LRUSize)

	fmt.Println("\n### Data Distribution")
	totalEntities := cfg.ServiceCount * cfg.InstancePerSvc
	fmt.Printf("- %d services, each with %d instances = %d total entities\n", cfg.ServiceCount, cfg.InstancePerSvc, totalEntities)
	fmt.Printf("- Entity composition: %s (from tags: %s)\n", fmt.Sprintf("%s+%s", cfg.EntityTags[0], cfg.EntityTags[1]), strings.Join(cfg.EntityTags, "+"))
	fmt.Printf("- Sharding by: %s (data distributed across %d shards)\n", strings.Join(cfg.ShardingKey, ", "), cfg.ShardNum)
	fmt.Printf("- `%s` has values ~100000+ to simulate a high-latency instance dominating topN\n", cfg.DominantEntity)
	fmt.Printf("- Other instances have values 1~%d (svc_idx*100 + inst_idx + bucket_idx)\n", (cfg.ServiceCount-1)*100+cfg.InstancePerSvc-1+cfg.TimeBucketCount)
	fmt.Printf("- %d time buckets, %s apart\n", cfg.TimeBucketCount, cfg.TimeBucketWidth)
	fmt.Printf("- Total data points: %d (entities x time_buckets)\n", totalEntities*cfg.TimeBucketCount)
	fmt.Printf("- Each time bucket stores up to %d entries per shard (counters_number)\n", cfg.CountersNumber)
	fmt.Printf("| Query Iterations | %d per scenario |\n", cfg.QueryIterations)
}

func printResultTable(results []BenchmarkResult) {
	fmt.Println("\n### Query Results")
	fmt.Println("| Scenario | TopN | Unique Entities | Total Items | Entity Details |")
	fmt.Println("|----------|------|-----------------|-------------|----------------|")
	for _, r := range results {
		fmt.Printf("| %s | %d | %d | %d | %s |\n",
			r.Scenario, r.TopN, r.UniqueEntities, r.TotalItems, r.EntityDetails)
	}
}

func printLatencyTable(results []BenchmarkResult) {
	fmt.Println("\n### Latency Statistics")
	fmt.Println("| Scenario | Min(ms) | Avg(ms) | P50(ms) | P95(ms) | P99(ms) | Max(ms) |")
	fmt.Println("|----------|---------|---------|---------|---------|---------|---------|")
	for _, r := range results {
		fmt.Printf("| %s | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f |\n",
			r.Scenario, r.Latency.Min, r.Latency.Avg, r.Latency.P50, r.Latency.P95, r.Latency.P99, r.Latency.Max)
	}
}

// WriteReport writes a Markdown report to a file.
func WriteReport(results []BenchmarkResult, cfg BenchmarkConfig, dir string) error {
	var sb strings.Builder

	sb.WriteString("# TopN Distributed Query Benchmark Report\n\n")

	// Environment section
	sb.WriteString("## Test Environment\n\n")
	sb.WriteString("| Item | Value |\n")
	sb.WriteString("|------|-------|\n")
	fmt.Fprintf(&sb, "| Deployment Mode | Distributed (1 liaison + %d data nodes) |\n", cfg.DataNodeCount)

	// Group configuration
	sb.WriteString("\n## Group Configuration\n\n")
	sb.WriteString("| Item | Value |\n")
	sb.WriteString("|------|-------|\n")
	fmt.Fprintf(&sb, "| Group | %s |\n", cfg.Group)
	sb.WriteString("| Catalog | CATALOG_MEASURE |\n")
	fmt.Fprintf(&sb, "| Shard Num | %d |\n", cfg.ShardNum)
	fmt.Fprintf(&sb, "| Segment Interval | %s |\n", cfg.SegmentInterval)
	fmt.Fprintf(&sb, "| TTL | %s |\n", cfg.TTL)

	// Measure schema
	sb.WriteString("\n## Measure Schema\n\n")
	sb.WriteString("| Item | Value |\n")
	sb.WriteString("|------|-------|\n")
	fmt.Fprintf(&sb, "| Measure | %s |\n", cfg.SourceMeasure)
	fmt.Fprintf(&sb, "| Group | %s |\n", cfg.Group)
	fmt.Fprintf(&sb, "| Entity Tags | %s |\n", strings.Join(cfg.EntityTags, ", "))
	fmt.Fprintf(&sb, "| Sharding Key | %s |\n", strings.Join(cfg.ShardingKey, ", "))
	fmt.Fprintf(&sb, "| Interval | %s |\n", cfg.TimeBucketWidth)
	sb.WriteString("| Tags | `instance_id` (string), `service_id` (string) |\n")
	sb.WriteString("| Fields | `value` (INT, Gorilla encoding, ZSTD compression) |\n")

	// TopNAggregation
	sb.WriteString("\n## TopNAggregation Configuration\n\n")
	sb.WriteString("| Item | Value |\n")
	sb.WriteString("|------|-------|\n")
	fmt.Fprintf(&sb, "| TopN Rule | %s |\n", cfg.TopNRuleName)
	fmt.Fprintf(&sb, "| Source Measure | %s |\n", cfg.SourceMeasure)
	sb.WriteString("| Field | `value` |\n")
	fmt.Fprintf(&sb, "| Group By Tags | %s (matches entity tags) |\n", strings.Join(cfg.EntityTags, ", "))
	fmt.Fprintf(&sb, "| Counters Number | %d |\n", cfg.CountersNumber)
	fmt.Fprintf(&sb, "| LRU Size | %d |\n", cfg.LRUSize)

	// Data distribution
	totalEntities := cfg.ServiceCount * cfg.InstancePerSvc
	sb.WriteString("\n## Data Distribution\n\n")
	fmt.Fprintf(&sb, "- **%d services**, each with **%d instances** = **%d total entities**\n", cfg.ServiceCount, cfg.InstancePerSvc, totalEntities)
	fmt.Fprintf(&sb, "- Entity composition: `%s` + `%s` (from entity tags)\n", cfg.EntityTags[0], cfg.EntityTags[1])
	fmt.Fprintf(&sb, "- Sharding by: `%s` (data distributed across %d shards)\n", strings.Join(cfg.ShardingKey, "`, `"), cfg.ShardNum)
	fmt.Fprintf(&sb, "- `%s` has values ~100000+ to simulate a high-latency instance dominating topN\n", cfg.DominantEntity)
	fmt.Fprintf(&sb, "- Other instances have values 1~%d (`svc_idx*100 + inst_idx + bucket_idx`)\n", (cfg.ServiceCount-1)*100+cfg.InstancePerSvc-1+cfg.TimeBucketCount)
	fmt.Fprintf(&sb, "- %d time buckets, %s apart\n", cfg.TimeBucketCount, cfg.TimeBucketWidth)
	fmt.Fprintf(&sb, "- Total data points: %d (entities × time_buckets)\n", totalEntities*cfg.TimeBucketCount)
	fmt.Fprintf(&sb, "- Each time bucket stores up to %d entries per shard (counters_number)\n", cfg.CountersNumber)

	sb.WriteString("\n## Query Scenarios\n\n")
	sb.WriteString("Each query covers the full time range (all time buckets), with different aggregation functions and topN values.\n\n")

	// Result table
	sb.WriteString("## Query Results\n\n")
	sb.WriteString("| Scenario | TopN | Unique Entities | Total Items | Entity Details |\n")
	sb.WriteString("|----------|------|-----------------|-------------|----------------|\n")
	for _, r := range results {
		fmt.Fprintf(&sb, "| %s | %d | %d | %d | %s |\n",
			r.Scenario, r.TopN, r.UniqueEntities, r.TotalItems, r.EntityDetails)
	}

	// Latency table
	sb.WriteString("\n## Latency Statistics\n\n")
	sb.WriteString("| Scenario | Min(ms) | Avg(ms) | P50(ms) | P95(ms) | P99(ms) | Max(ms) |\n")
	sb.WriteString("|----------|---------|---------|---------|---------|---------|---------|\n")
	for _, r := range results {
		fmt.Fprintf(&sb, "| %s | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f |\n",
			r.Scenario, r.Latency.Min, r.Latency.Avg, r.Latency.P50, r.Latency.P95, r.Latency.P99, r.Latency.Max)
	}

	// Analysis section
	sb.WriteString("\n## Analysis\n\n")
	sb.WriteString("### Expected vs Actual Behavior\n\n")
	sb.WriteString("When one entity (`svc_0/inst_0`) dominates with significantly higher values:\n\n")
	sb.WriteString("1. **Data node level**: Each data node's `top` operation (`measure_plan_top.go`) selects topN globally across ALL time buckets, not per time bucket\n")
	fmt.Fprintf(&sb, "2. **Result**: If topN=3 and `svc_0/inst_0` has the top values in all %d time buckets, the data node returns 3 entries all for `svc_0/inst_0`\n", cfg.TimeBucketCount)
	sb.WriteString("3. **Coordinator level**: The PostProcessor aggregates with MEAN/SUM/COUNT, but only sees `svc_0/inst_0` entries — other entities are already lost\n\n")
	sb.WriteString("This is the root cause of incorrect results for SUM/COUNT/MEAN in distributed mode.\n")
	sb.WriteString("MAX/MIN are unaffected because the dominant entity's max/min value is the same regardless of aggregation.\n")

	path := filepath.Join(dir, "topn_benchmark_report.md")
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}
