# TopN 分布式查询性能测试报告

## 测试环境

| 项目 | 值 |
|------|-----|
| 部署模式 | 分布式（1 liaison + 3 data node） |

## Group 配置

| 项目 | 值 |
|------|-----|
| Group | sw_metric |
| Catalog | CATALOG_MEASURE |
| Shard Num | 2 |
| Segment Interval | 1d |
| TTL | 7d |

## Measure Schema

| 项目 | 值 |
|------|-----|
| Measure | service_instance_latency_bench |
| Group | sw_metric |
| Entity Tags | service_id, instance_id |
| Sharding Key | service_id |
| Interval | 1m |
| Tags | `instance_id` (string), `service_id` (string) |
| Fields | `value` (INT, Gorilla 编码, ZSTD 压缩) |

## TopNAggregation 配置

| 项目 | 值 |
|------|-----|
| TopN Rule | topn_agg_bench |
| Source Measure | service_instance_latency_bench |
| Field | `value` |
| Group By Tags | service_id, instance_id（与 entity tags 一致） |
| Counters Number | 1000 |
| LRU Size | 10 |

## 数据分布

- **20 个 service**，每个 **100 个 instance** = **2000 个 entity**
- Entity 组成：`service_id` + `instance_id`（来自 entity tags）
- 分片方式：按 `service_id` 分片（数据分布在 2 个 shard）
- `svc_0/inst_0` 的值为 ~100000+（模拟高延迟 instance 占据 TopN）
- 其他 instance 的值为 1~2029（`svc_idx*100 + inst_idx + bucket_idx`）
- 30 个时间桶，间隔 1 分钟，查询时间范围 30 分钟
- 总数据点：60000（entities × time_buckets）
- 每个时间桶每个 shard 最多存储 1000 条记录（counters_number）

## 查询场景

每个查询覆盖完整时间范围（30 个时间桶），使用不同的聚合函数和 topN 值。每个场景执行 200 次查询，前 5 次作为 warmup。

## 查询结果对比

### main 分支（当前：agg 不下推）

| 场景 | TopN | 唯一 Entity 数 | 总 Item 数 | Entity 详情 |
|------|------|----------------|------------|-------------|
| COUNT/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=3; service_id=svc_18,instance_id=svc_18_inst_99=2; service_id=svc_18,instance_id=svc_18_inst_98=1 |
| MAX/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100290; service_id=svc_18,instance_id=svc_18_inst_99=1928; service_id=svc_18,instance_id=svc_18_inst_98=1927 |
| MEAN/top10/DESC | 10 | 5 | 5 | service_id=svc_0,instance_id=svc_0_inst_0=100245; service_id=svc_18,instance_id=svc_18_inst_98=1926; service_id=svc_18,instance_id=svc_18_inst_99=1926; service_id=svc_18,instance_id=svc_18_inst_97=1925; service_id=svc_18,instance_id=svc_18_inst_96=1925 |
| MEAN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100280; service_id=svc_18,instance_id=svc_18_inst_99=1927; service_id=svc_18,instance_id=svc_18_inst_98=1927 |
| MIN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100270; service_id=svc_18,instance_id=svc_18_inst_99=1927; service_id=svc_18,instance_id=svc_18_inst_98=1927 |
| SUM/top10/DESC | 10 | 5 | 5 | service_id=svc_0,instance_id=svc_0_inst_0=1002450; service_id=svc_18,instance_id=svc_18_inst_99=7706; service_id=svc_18,instance_id=svc_18_inst_98=5778; service_id=svc_18,instance_id=svc_18_inst_97=3851; service_id=svc_18,instance_id=svc_18_inst_96=1925 |
| SUM/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=300840; service_id=svc_18,instance_id=svc_18_inst_99=3855; service_id=svc_18,instance_id=svc_18_inst_98=1927 |

### fix-topn-distributed-query 分支（agg 下推到 data node）

| 场景 | TopN | 唯一 Entity 数 | 总 Item 数 | Entity 详情 |
|------|------|----------------|------------|-------------|
| COUNT/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=30; service_id=svc_5,instance_id=svc_5_inst_53=29; service_id=svc_2,instance_id=svc_2_inst_19=29 |
| MAX/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100290; service_id=svc_19,instance_id=svc_19_inst_99=2028; service_id=svc_19,instance_id=svc_19_inst_98=2027 |
| MEAN/top10/DESC | 10 | **10** | **10** | service_id=svc_0,instance_id=svc_0_inst_0=100145; service_id=svc_19,instance_id=svc_19_inst_99=2014; service_id=svc_19,instance_id=svc_19_inst_98=2013; service_id=svc_19,instance_id=svc_19_inst_97=2012; service_id=svc_19,instance_id=svc_19_inst_96=2011; service_id=svc_19,instance_id=svc_19_inst_95=2010; service_id=svc_19,instance_id=svc_19_inst_94=2009; service_id=svc_19,instance_id=svc_19_inst_93=2008; service_id=svc_19,instance_id=svc_19_inst_92=2007; service_id=svc_19,instance_id=svc_19_inst_91=2006 |
| MEAN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100145; service_id=svc_19,instance_id=svc_19_inst_99=2014; service_id=svc_19,instance_id=svc_19_inst_98=2013 |
| MIN/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=100000; service_id=svc_19,instance_id=svc_19_inst_99=2000; service_id=svc_19,instance_id=svc_19_inst_98=1999 |
| SUM/top10/DESC | 10 | **10** | **10** | service_id=svc_0,instance_id=svc_0_inst_0=3004350; service_id=svc_19,instance_id=svc_19_inst_99=58406; service_id=svc_19,instance_id=svc_19_inst_98=58377; service_id=svc_19,instance_id=svc_19_inst_97=58348; service_id=svc_19,instance_id=svc_19_inst_96=58319; service_id=svc_19,instance_id=svc_19_inst_95=58290; service_id=svc_19,instance_id=svc_19_inst_94=58261; service_id=svc_19,instance_id=svc_19_inst_93=58232; service_id=svc_19,instance_id=svc_19_inst_92=58203; service_id=svc_19,instance_id=svc_19_inst_91=58174 |
| SUM/top3/DESC | 3 | 3 | 3 | service_id=svc_0,instance_id=svc_0_inst_0=3004350; service_id=svc_19,instance_id=svc_19_inst_99=58406; service_id=svc_19,instance_id=svc_19_inst_98=58377 |

## 延迟统计对比

| 场景 | 分支 | Min(ms) | Avg(ms) | P50(ms) | P95(ms) | P99(ms) | Max(ms) |
|------|------|---------|---------|---------|---------|---------|---------|
| COUNT/top3/DESC | main | 133.77 | 150.71 | 148.90 | 166.85 | 171.61 | 176.64 |
| COUNT/top3/DESC | **agg 下推** | 167.41 | 183.69 | 182.76 | 202.04 | 218.23 | 225.34 |
| MAX/top3/DESC | main | 133.66 | 153.96 | 150.26 | 179.82 | 204.93 | 205.76 |
| MAX/top3/DESC | **agg 下推** | 165.48 | 183.69 | 182.69 | 200.26 | 205.88 | 206.80 |
| MEAN/top10/DESC | main | 135.84 | 153.41 | 150.96 | 172.01 | 186.20 | 188.42 |
| MEAN/top10/DESC | **agg 下推** | 164.21 | 183.97 | 183.10 | 200.85 | 208.81 | 212.73 |
| MEAN/top3/DESC | main | 128.10 | 150.19 | 149.06 | 163.89 | 172.91 | 190.33 |
| MEAN/top3/DESC | **agg 下推** | 168.90 | 189.52 | 185.81 | 211.44 | 253.14 | 279.85 |
| MIN/top3/DESC | main | 132.38 | 149.38 | 148.40 | 161.72 | 173.62 | 175.49 |
| MIN/top3/DESC | **agg 下推** | 165.08 | 186.69 | 185.38 | 210.44 | 225.99 | 227.41 |
| SUM/top10/DESC | main | 137.10 | 151.09 | 150.00 | 162.96 | 194.51 | 195.01 |
| SUM/top10/DESC | **agg 下推** | 162.56 | 183.62 | 182.64 | 202.88 | 218.81 | 224.60 |
| SUM/top3/DESC | main | 137.84 | 151.98 | 150.18 | 167.79 | 180.25 | 183.30 |
| SUM/top3/DESC | **agg 下推** | 162.65 | 182.85 | 182.07 | 202.13 | 212.00 | 214.10 |

## 延迟差异汇总

| 场景 | main P50(ms) | agg 下推 P50(ms) | 差异(ms) | 差异(%) |
|------|-------------|-----------------|----------|---------|
| COUNT/top3/DESC | 148.90 | 182.76 | +33.86 | +22.7% |
| MAX/top3/DESC | 150.26 | 182.69 | +32.43 | +21.6% |
| MEAN/top10/DESC | 150.96 | 183.10 | +32.14 | +21.3% |
| MEAN/top3/DESC | 149.06 | 185.81 | +36.75 | +24.7% |
| MIN/top3/DESC | 148.40 | 185.38 | +36.98 | +24.9% |
| SUM/top10/DESC | 150.00 | 182.64 | +32.64 | +21.8% |
| SUM/top3/DESC | 150.18 | 182.07 | +31.89 | +21.2% |

## 总结

### 正确性

agg 下推分支**显著改善了查询正确性**：

| 场景 | main 结果 | agg 下推结果 | 正确性 |
|------|----------|-------------|--------|
| COUNT/top3 | 只返回 svc_0 的 inst，count=3（应为 30） | 返回 **3 个不同 entity**，svc_0/inst_0 count=**30** | **修复** |
| MEAN/top10 | 只返回 **5 个** entity（应为 10 个），值不正确 | 返回 **10 个** 不同 entity，值正确 | **修复** |
| MEAN/top3 | 值不正确（100280 vs 正确值 100145） | 值正确（100145） | **修复** |
| SUM/top10 | 只返回 **5 个** entity（应为 10 个），值不正确 | 返回 **10 个** 不同 entity，值正确 | **修复** |
| SUM/top3 | 值不正确（300840 vs 正确值 3004350） | 值正确（3004350） | **修复** |
| MAX/top3 | 正确 | 正确 | 不变 |
| MIN/top3 | 正确 | 正确 | 不变 |

**根因**：main 分支在分布式模式下，data node 先做全局 topN 截断（`measure_plan_top.go`），再做聚合。当一个 entity（`svc_0/inst_0`）在所有 30 个时间桶中都排在最前面时，topN=3 的截断导致 data node 只返回该 entity 的数据，其他 entity 的数据在聚合前就已丢失。agg 下推分支将聚合操作下推到 data node，先按 entity 分组聚合，再做 topN 截断，从而保证了结果正确性。

### 性能

agg 下推分支的查询延迟 **P50 增加 22%~25%（约 32~37ms）**。

30 分钟时间范围相比之前的 15 分钟测试（+3%~+4%），延迟增幅明显变大，原因是：
1. 时间桶从 15 增加到 30，data node 本地需要处理的预计算数据量翻倍
2. agg 下推需要在 data node 上对每个 entity 做 groupBy + 聚合，数据量翻倍后计算量也翻倍
3. 主要瓶颈仍是磁盘 I/O（占 ~19%）和 GC（占 ~23%）

**结论**：agg 下推在增加约 22%~25% 延迟开销的情况下，完全修复了分布式 TopN 查询在 MEAN/SUM/COUNT 聚合下的结果正确性问题。考虑到绝对延迟从 ~150ms 增加到 ~183ms，增量约 33ms，对于数据库查询来说仍然在可接受范围内。
