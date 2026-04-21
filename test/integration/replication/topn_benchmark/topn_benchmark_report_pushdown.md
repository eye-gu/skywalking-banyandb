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
| COUNT/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=30; service_id=svc_5,instance_id=svc_5_inst_53=29; service_id=svc_2,instance_id=svc_2_inst_19=29 |
| MAX/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100290; service_id=svc_19,instance_id=svc_19_inst_99=2028; service_id=svc_19,instance_id=svc_19_inst_98=2027 |
| MEAN/top10/DESC | 10 | 10 | 10 | service_id=svc_0,instance_id=svc_0_inst_0=100145; service_id=svc_19,instance_id=svc_19_inst_99=2014; service_id=svc_19,instance_id=svc_19_inst_98=2013; service_id=svc_19,instance_id=svc_19_inst_97=2012; service_id=svc_19,instance_id=svc_19_inst_96=2011; service_id=svc_19,instance_id=svc_19_inst_95=2010; service_id=svc_19,instance_id=svc_19_inst_94=2009; service_id=svc_19,instance_id=svc_19_inst_93=2008; service_id=svc_19,instance_id=svc_19_inst_92=2007; service_id=svc_19,instance_id=svc_19_inst_91=2006 |
| MEAN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100145; service_id=svc_19,instance_id=svc_19_inst_99=2014; service_id=svc_19,instance_id=svc_19_inst_98=2013 |
| MIN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100000; service_id=svc_19,instance_id=svc_19_inst_99=2000; service_id=svc_19,instance_id=svc_19_inst_98=1999 |
| SUM/top10/DESC | 10 | 10 | 10 | service_id=svc_0,instance_id=svc_0_inst_0=3004350; service_id=svc_19,instance_id=svc_19_inst_99=58406; service_id=svc_19,instance_id=svc_19_inst_98=58377; service_id=svc_19,instance_id=svc_19_inst_97=58348; service_id=svc_19,instance_id=svc_19_inst_96=58319; service_id=svc_19,instance_id=svc_19_inst_95=58290; service_id=svc_19,instance_id=svc_19_inst_94=58261; service_id=svc_19,instance_id=svc_19_inst_93=58232; service_id=svc_19,instance_id=svc_19_inst_92=58203; service_id=svc_19,instance_id=svc_19_inst_91=58174 |
| SUM/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=3004350; service_id=svc_19,instance_id=svc_19_inst_99=58406; service_id=svc_19,instance_id=svc_19_inst_98=58377 |

## Latency Statistics

| Scenario | Min(ms) | Avg(ms) | P50(ms) | P95(ms) | P99(ms) | Max(ms) |
|----------|---------|---------|---------|---------|---------|---------|
| COUNT/top3/DESC | 167.41 | 183.69 | 182.76 | 202.04 | 218.23 | 225.34 |
| MAX/top3/DESC | 165.48 | 183.69 | 182.69 | 200.26 | 205.88 | 206.80 |
| MEAN/top10/DESC | 164.21 | 183.97 | 183.10 | 200.85 | 208.81 | 212.73 |
| MEAN/top3/DESC | 168.90 | 189.52 | 185.81 | 211.44 | 253.14 | 279.85 |
| MIN/top3/DESC | 165.08 | 186.69 | 185.38 | 210.44 | 225.99 | 227.41 |
| SUM/top10/DESC | 162.56 | 183.62 | 182.64 | 202.88 | 218.81 | 224.60 |
| SUM/top3/DESC | 162.65 | 182.85 | 182.07 | 202.13 | 212.00 | 214.10 |

## Analysis

### Expected vs Actual Behavior

When one entity (`svc_0/inst_0`) dominates with significantly higher values:

1. **Data node level**: Each data node's `top` operation (`measure_plan_top.go`) selects topN globally across ALL time buckets, not per time bucket
2. **Result**: If topN=3 and `svc_0/inst_0` has the top values in all 30 time buckets, the data node returns 3 entries all for `svc_0/inst_0`
3. **Coordinator level**: The PostProcessor aggregates with MEAN/SUM/COUNT, but only sees `svc_0/inst_0` entries — other entities are already lost

This is the root cause of incorrect results for SUM/COUNT/MEAN in distributed mode.
MAX/MIN are unaffected because the dominant entity's max/min value is the same regardless of aggregation.
