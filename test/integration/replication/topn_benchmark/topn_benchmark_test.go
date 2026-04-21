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
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/common/v1"
	measurev1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/measure/v1"
	modelv1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/model/v1"
)

const (
	measureName  = "service_instance_latency_bench"
	groupName    = "sw_metric"
	topNRuleName = "topn_agg_bench"
	tagFamily    = "default"

	serviceCount    = 20
	instancePerSvc  = 100
	timeBucketCount = 30
	queryIterations = 200
)

// writeTopNData writes measure data that triggers TopN pre-computation.
// It creates serviceCount services, each with instancePerSvc instances,
// across timeBucketCount time buckets (1 min apart).
// Instance "svc_0/inst_0" has significantly higher values to test the "one entity dominates" scenario.
func writeTopNData(conn *grpc.ClientConn, base time.Time) {
	client := measurev1.NewMeasureServiceClient(conn)
	ctx := context.Background()

	stream, streamErr := client.Write(ctx)
	Expect(streamErr).NotTo(HaveOccurred())

	recvErrCh := make(chan error, 1)
	go func() {
		for {
			resp, recvErr := stream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					recvErrCh <- nil
					return
				}
				recvErrCh <- recvErr
				return
			}
			if resp.GetStatus() != modelv1.Status_STATUS_SUCCEED.String() {
				recvErrCh <- fmt.Errorf("write failed: %s", resp.GetStatus())
				return
			}
		}
	}()

	metadata := &commonv1.Metadata{Name: measureName, Group: groupName}
	spec := &measurev1.DataPointSpec{
		TagFamilySpec: []*measurev1.TagFamilySpec{{
			Name:     tagFamily,
			TagNames: []string{"instance_id", "service_id"},
		}},
		FieldNames: []string{"value"},
	}

	messageID := uint64(1)
	sent := 0
	totalPoints := serviceCount * instancePerSvc * timeBucketCount
	for bucketIdx := range timeBucketCount {
		ts := base.Add(-time.Duration(timeBucketCount-bucketIdx-1) * time.Minute)
		for svcIdx := range serviceCount {
			serviceID := fmt.Sprintf("svc_%d", svcIdx)
			for instIdx := range instancePerSvc {
				instanceID := fmt.Sprintf("svc_%d_inst_%d", svcIdx, instIdx)

				var value int64
				// svc_0/inst_0: dominant entity with very high latency
				if svcIdx == 0 && instIdx == 0 {
					value = int64(100000 + bucketIdx*10)
				} else {
					value = int64(svcIdx*100+instIdx + bucketIdx + 1)
				}

				req := &measurev1.WriteRequest{
					Metadata:      metadata,
					DataPointSpec: spec,
					DataPoint: &measurev1.DataPointValue{
						Timestamp: timestamppb.New(ts),
						TagFamilies: []*modelv1.TagFamilyForWrite{{
							Tags: []*modelv1.TagValue{
								{Value: &modelv1.TagValue_Str{Str: &modelv1.Str{Value: instanceID}}},
								{Value: &modelv1.TagValue_Str{Str: &modelv1.Str{Value: serviceID}}},
							},
						}},
						Fields: []*modelv1.FieldValue{{
							Value: &modelv1.FieldValue_Int{Int: &modelv1.Int{Value: value}},
						}},
					},
					MessageId: messageID,
				}
				Expect(stream.Send(req)).To(Succeed())
				messageID++
				metadata = nil
				spec = nil
				sent++
				if sent%3000 == 0 {
					By(fmt.Sprintf("Write progress: %d/%d data points sent", sent, totalPoints))
				}
			}
		}
	}

	Expect(stream.CloseSend()).To(Succeed())
	Expect(<-recvErrCh).NotTo(HaveOccurred())
}

// queryScenario defines a benchmark query configuration.
type queryScenario struct {
	name string
	agg  modelv1.AggregationFunction
	topN int32
	sort modelv1.Sort
}

// runTopNQuery executes a TopN query and returns the response and duration.
func runTopNQuery(ctx context.Context, client measurev1.MeasureServiceClient,
	query *measurev1.TopNRequest,
) (*measurev1.TopNResponse, time.Duration, error) {
	start := time.Now()
	resp, err := client.TopN(ctx, query)
	elapsed := time.Since(start)
	if err != nil {
		return nil, elapsed, err
	}
	return resp, elapsed, nil
}

// extractEntityKey builds a readable key from entity tags: "service_id=svc_0,instance_id=svc_0_inst_0".
func extractEntityKey(tags []*modelv1.Tag) string {
	var keyBuilder strings.Builder
	for _, tag := range tags {
		if tag.GetValue() != nil {
			if strVal := tag.GetValue().GetStr(); strVal != nil {
				if keyBuilder.Len() > 0 {
					keyBuilder.WriteByte(',')
				}
				keyBuilder.WriteString(tag.GetKey())
				keyBuilder.WriteByte('=')
				keyBuilder.WriteString(strVal.GetValue())
			}
		}
	}
	return keyBuilder.String()
}

// extractEntitySummary extracts a summary of entity IDs and values from a TopN response.
func extractEntitySummary(resp *measurev1.TopNResponse, maxShow int) string {
	var items []string
	for _, list := range resp.GetLists() {
		for idx, item := range list.GetItems() {
			if maxShow > 0 && idx >= maxShow {
				items = append(items, "...")
				break
			}
			entityKey := extractEntityKey(item.GetEntity())
			val := item.GetValue().GetInt().GetValue()
			items = append(items, fmt.Sprintf("%s=%d", entityKey, val))
		}
	}
	return strings.Join(items, "; ")
}

// countUniqueEntities counts unique entities in a TopN response.
func countUniqueEntities(resp *measurev1.TopNResponse) int {
	seen := make(map[string]bool)
	for _, list := range resp.GetLists() {
		for _, item := range list.GetItems() {
			seen[extractEntityKey(item.GetEntity())] = true
		}
	}
	return len(seen)
}

// countTotalItems counts total items in a TopN response.
func countTotalItems(resp *measurev1.TopNResponse) int {
	total := 0
	for _, list := range resp.GetLists() {
		total += len(list.GetItems())
	}
	return total
}

var _ = Describe("TopN Benchmark", func() {
	scenarios := []queryScenario{
		{name: "MEAN/top3/DESC", agg: modelv1.AggregationFunction_AGGREGATION_FUNCTION_MEAN, topN: 3, sort: modelv1.Sort_SORT_DESC},
		{name: "SUM/top3/DESC", agg: modelv1.AggregationFunction_AGGREGATION_FUNCTION_SUM, topN: 3, sort: modelv1.Sort_SORT_DESC},
		{name: "COUNT/top3/DESC", agg: modelv1.AggregationFunction_AGGREGATION_FUNCTION_COUNT, topN: 3, sort: modelv1.Sort_SORT_DESC},
		{name: "MAX/top3/DESC", agg: modelv1.AggregationFunction_AGGREGATION_FUNCTION_MAX, topN: 3, sort: modelv1.Sort_SORT_DESC},
		{name: "MIN/top3/DESC", agg: modelv1.AggregationFunction_AGGREGATION_FUNCTION_MIN, topN: 3, sort: modelv1.Sort_SORT_DESC},
		{name: "MEAN/top10/DESC", agg: modelv1.AggregationFunction_AGGREGATION_FUNCTION_MEAN, topN: 10, sort: modelv1.Sort_SORT_DESC},
		{name: "SUM/top10/DESC", agg: modelv1.AggregationFunction_AGGREGATION_FUNCTION_SUM, topN: 10, sort: modelv1.Sort_SORT_DESC},
	}

	var results []BenchmarkResult

	for _, scenario := range scenarios {
		It(fmt.Sprintf("benchmarks %s", scenario.name), func() {
			ctx := context.Background()

			query := &measurev1.TopNRequest{
				Name:   topNRuleName,
				Groups: []string{groupName},
				TimeRange: &modelv1.TimeRange{
					Begin: timestamppb.New(baseTime.Add(-time.Duration(timeBucketCount) * time.Minute)),
					End:   timestamppb.New(baseTime.Add(time.Minute)),
				},
				TopN:           scenario.topN,
				FieldValueSort: scenario.sort,
				Agg:            scenario.agg,
			}

			client := measurev1.NewMeasureServiceClient(distributedConn)

			// Warmup
			for range 5 {
				_, _, _ = runTopNQuery(ctx, client, query)
			}

			// Benchmark
			durations := make([]time.Duration, 0, queryIterations)
			var firstResp *measurev1.TopNResponse
			for i := range queryIterations {
				resp, elapsed, queryErr := runTopNQuery(ctx, client, query)
				Expect(queryErr).NotTo(HaveOccurred())
				durations = append(durations, elapsed)
				if i == 0 {
					firstResp = resp
				}
			}

			uniqueEntities := countUniqueEntities(firstResp)
			totalItems := countTotalItems(firstResp)
			entityDetails := extractEntitySummary(firstResp, 0)

			result := BenchmarkResult{
				Scenario:       scenario.name,
				AggFunc:        scenario.agg.String(),
				TopN:           scenario.topN,
				Sort:           scenario.sort.String(),
				Latency:        ComputeLatencyStats(durations),
				UniqueEntities: uniqueEntities,
				TotalItems:     totalItems,
				EntityDetails:  entityDetails,
			}
			results = append(results, result)

			By(fmt.Sprintf("%s: %d unique entities, %d total items, P50=%.2fms",
				scenario.name, uniqueEntities, totalItems, result.Latency.P50))
		})
	}

	AfterEach(func() {
		if len(results) > 0 {
			cfg := BenchmarkConfig{
				DataNodeCount:   dataNodeCount,
				ShardNum:        2,
				ServiceCount:    serviceCount,
				InstancePerSvc:  instancePerSvc,
				TimeBucketCount: timeBucketCount,
				TimeBucketWidth: "1m",
				DominantEntity:  "svc_0/inst_0",
				TopNRuleName:    topNRuleName,
				SourceMeasure:   measureName,
				Group:           groupName,
				EntityTags:      []string{"service_id", "instance_id"},
				ShardingKey:     []string{"service_id"},
				LRUSize:         10,
				CountersNumber:  1000,
				QueryIterations: queryIterations,
				SegmentInterval: "1d",
				TTL:             "7d",
			}
			PrintReport(results, cfg)
			writeErr := WriteReport(results, cfg, ".")
			if writeErr == nil {
				By("Report written to topn_benchmark_report.md")
			}
		}
	})
})
