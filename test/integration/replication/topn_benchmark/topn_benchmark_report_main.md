# TopN Distributed Query Benchmark Report

## Test Environment

| Item | Value |
|------|-------|
| Deployment Mode | Distributed (1 liaison + 3 data nodes) |

## Group Configuration

| Item | Value |
|------|-------|
| Group | sw_metric |
| Catalog | CATALOG_MEASURE |
| Shard Num | 2 |
| Segment Interval | 1d |
| TTL | 7d |

## Measure Schema

| Item | Value |
|------|-------|
| Measure | service_instance_latency_bench |
| Group | sw_metric |
| Entity Tags | service_id, instance_id |
| Sharding Key | service_id |
| Interval | 1m |
| Tags | `instance_id` (string), `service_id` (string) |
| Fields | `value` (INT, Gorilla encoding, ZSTD compression) |

## TopNAggregation Configuration

| Item | Value |
|------|-------|
| TopN Rule | topn_agg_bench |
| Source Measure | service_instance_latency_bench |
| Field | `value` |
| Group By Tags | service_id, instance_id (matches entity tags) |
| Counters Number | 1000 |
| LRU Size | 10 |

## Data Distribution

- **20 services**, each with **100 instances** = **2000 total entities**
- Entity composition: `service_id` + `instance_id` (from entity tags)
- Sharding by: `service_id` (data distributed across 2 shards)
- `svc_0/inst_0` has values ~100000+ to simulate a high-latency instance dominating topN
- Other instances have values 1~2029 (`svc_idx*100 + inst_idx + bucket_idx`)
- 30 time buckets, 1m apart
- Total data points: 60000 (entities × time_buckets)
- Each time bucket stores up to 1000 entries per shard (counters_number)

## Query Scenarios

Each query covers the full time range (all time buckets), with different aggregation functions and topN values.

## Query Results

| Scenario | TopN | Unique Entities | Total Items | Entity Details |
|----------|------|-----------------|-------------|----------------|
| COUNT/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=3; service_id=svc_18,instance_id=svc_18_inst_99=2; service_id=svc_18,instance_id=svc_18_inst_98=1 |
| MAX/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100290; service_id=svc_18,instance_id=svc_18_inst_99=1928; service_id=svc_18,instance_id=svc_18_inst_98=1927 |
| MEAN/top10/DESC | 10 | 5 | 5 | service_id=svc_0,instance_id=svc_0_inst_0=100245; service_id=svc_18,instance_id=svc_18_inst_98=1926; service_id=svc_18,instance_id=svc_18_inst_99=1926; service_id=svc_18,instance_id=svc_18_inst_97=1925; service_id=svc_18,instance_id=svc_18_inst_96=1925 |
| MEAN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100280; service_id=svc_18,instance_id=svc_18_inst_99=1927; service_id=svc_18,instance_id=svc_18_inst_98=1927 |
| MIN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100270; service_id=svc_18,instance_id=svc_18_inst_99=1927; service_id=svc_18,instance_id=svc_18_inst_98=1927 |
| SUM/top10/DESC | 10 | 5 | 5 | service_id=svc_0,instance_id=svc_0_inst_0=1002450; service_id=svc_18,instance_id=svc_18_inst_99=7706; service_id=svc_18,instance_id=svc_18_inst_98=5778; service_id=svc_18,instance_id=svc_18_inst_97=3851; service_id=svc_18,instance_id=svc_18_inst_96=1925 |
| SUM/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=300840; service_id=svc_18,instance_id=svc_18_inst_99=3855; service_id=svc_18,instance_id=svc_18_inst_98=1927 |

## Latency Statistics

| Scenario | Min(ms) | Avg(ms) | P50(ms) | P95(ms) | P99(ms) | Max(ms) |
|----------|---------|---------|---------|---------|---------|---------|
| COUNT/top3/DESC | 133.77 | 150.71 | 148.90 | 166.85 | 171.61 | 176.64 |
| MAX/top3/DESC | 133.66 | 153.96 | 150.26 | 179.82 | 204.93 | 205.76 |
| MEAN/top10/DESC | 135.84 | 153.41 | 150.96 | 172.01 | 186.20 | 188.42 |
| MEAN/top3/DESC | 128.10 | 150.19 | 149.06 | 163.89 | 172.91 | 190.33 |
| MIN/top3/DESC | 132.38 | 149.38 | 148.40 | 161.72 | 173.62 | 175.49 |
| SUM/top10/DESC | 137.10 | 151.09 | 150.00 | 162.96 | 194.51 | 195.01 |
| SUM/top3/DESC | 137.84 | 151.98 | 150.18 | 167.79 | 180.25 | 183.30 |

## Analysis

### Expected vs Actual Behavior

When one entity (`svc_0/inst_0`) dominates with significantly higher values:

1. **Data node level**: Each data node's `top` operation (`measure_plan_top.go`) selects topN globally across ALL time buckets, not per time bucket
2. **Result**: If topN=3 and `svc_0/inst_0` has the top values in all 30 time buckets, the data node returns 3 entries all for `svc_0/inst_0`
3. **Coordinator level**: The PostProcessor aggregates with MEAN/SUM/COUNT, but only sees `svc_0/inst_0` entries — other entities are already lost

This is the root cause of incorrect results for SUM/COUNT/MEAN in distributed mode.
MAX/MIN are unaffected because the dominant entity's max/min value is the same regardless of aggregation.
