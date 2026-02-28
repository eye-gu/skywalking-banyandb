// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
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

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/common/v1"
	databasev1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/database/v1"
	modelv1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/model/v1"
	streamv1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/stream/v1"
	"github.com/apache/skywalking-banyandb/pkg/timestamp"
)

const (
	serverAddr = "localhost:17912"
	group      = "stream_group"
	streamName = "float3"
)

func main() {
	// 建立gRPC连接
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()

	// // 1. 创建Group
	// if err := createGroup(ctx, conn); err != nil {
	// 	log.Fatalf("Failed to create group: %v", err)
	// }
	// fmt.Println("Group created successfully")

	// // 2. 创建Stream（包含float64类型的tag）
	// if err := createStream(ctx, conn); err != nil {
	// 	log.Fatalf("Failed to create stream: %v", err)
	// }
	// fmt.Println("Stream created successfully")

	// 3. 写入测试数据（包含float64值）
	if err := writeData(ctx, conn); err != nil {
		log.Fatalf("Failed to write data: %v", err)
	}
	fmt.Println("Data written successfully")

	// // 4. 查询数据
	// if err := queryData(ctx, conn); err != nil {
	// 	log.Fatalf("Failed to query data: %v", err)
	// }
	// fmt.Println("Data queried successfully")
}

// createGroup 创建stream group
func createGroup(ctx context.Context, conn *grpc.ClientConn) error {
	client := databasev1.NewGroupRegistryServiceClient(conn)

	req := &databasev1.GroupRegistryServiceCreateRequest{
		Group: &commonv1.Group{
			Metadata: &commonv1.Metadata{
				Name: group,
			},
			Catalog: commonv1.Catalog_CATALOG_STREAM,
			ResourceOpts: &commonv1.ResourceOpts{
				ShardNum: 1,
				SegmentInterval: &commonv1.IntervalRule{
					Unit: commonv1.IntervalRule_UNIT_HOUR,
					Num:  1,
				},
				Ttl: &commonv1.IntervalRule{
					Unit: commonv1.IntervalRule_UNIT_DAY,
					Num:  1,
				},
			},
		},
	}

	_, err := client.Create(ctx, req)
	return err
}

// createStream 创建包含float64类型tag的stream
func createStream(ctx context.Context, conn *grpc.ClientConn) error {
	client := databasev1.NewStreamRegistryServiceClient(conn)

	req := &databasev1.StreamRegistryServiceCreateRequest{
		Stream: &databasev1.Stream{
			Metadata: &commonv1.Metadata{
				Name:  streamName,
				Group: group,
			},
			TagFamilies: []*databasev1.TagFamilySpec{
				{
					Name: "searchable",
					Tags: []*databasev1.TagSpec{
						{
							Name: "stream_id",
							Type: databasev1.TagType_TAG_TYPE_STRING,
						},
						{
							Name: "trace_id",
							Type: databasev1.TagType_TAG_TYPE_STRING,
						},
						{
							Name: "value",
							//Type: databasev1.TagType_TAG_TYPE_FLOAT,
						},
					},
				},
			},
			Entity: &databasev1.Entity{
				TagNames: []string{"stream_id"},
			},
		},
	}

	_, err := client.Create(ctx, req)
	return err
}

// writeData 写入包含float64值的测试数据
func writeData(ctx context.Context, conn *grpc.ClientConn) error {
	client := streamv1.NewStreamServiceClient(conn)

	// 创建写入流
	writeStream, err := client.Write(ctx)
	if err != nil {
		return fmt.Errorf("failed to create write stream: %w", err)
	}

	// 写入几条测试数据
	for i := 0; i < 5; i++ {
		req := &streamv1.WriteRequest{
			Metadata: &commonv1.Metadata{
				Name:  streamName,
				Group: group,
			},
			Element: &streamv1.ElementValue{
				ElementId: fmt.Sprintf("element_%d", i),
				Timestamp: timestamppb.New(timestamp.NowMilli().Add(time.Duration(i) * time.Second)),
				//TagFamilies: []*modelv1.TagFamilyForWrite{
				//	{
				//		Tags: []*modelv1.TagValue{
				//			{
				//				Value: &modelv1.TagValue_Str{
				//					Str: &modelv1.Str{
				//						Value: fmt.Sprintf("stream_%d", i%2),
				//					},
				//				},
				//			},
				//			{
				//				Value: &modelv1.TagValue_Str{
				//					Str: &modelv1.Str{
				//						Value: fmt.Sprintf("trace_%d", i),
				//					},
				//				},
				//			},
				//			{
				//				Value: &modelv1.TagValue_Float{
				//					Float: &modelv1.Float{
				//						Value: float64(i)*3.14 - 5,
				//					},
				//				},
				//			},
				//			//{
				//			//	Value: &modelv1.TagValue_Int{
				//			//		Int: &modelv1.Int{
				//			//			Value: int64(i) - 2,
				//			//		},
				//			//	},
				//			//},
				//		},
				//	},
				//},
				TagFamilies: []*modelv1.TagFamilyForWrite{
					{
						Tags: []*modelv1.TagValue{
							{
								Value: &modelv1.TagValue_Str{
									Str: &modelv1.Str{
										Value: fmt.Sprintf("trace_%d", i),
									},
								},
							},
						},
					},
					{
						Tags: []*modelv1.TagValue{
							{
								Value: &modelv1.TagValue_Str{
									Str: &modelv1.Str{
										Value: fmt.Sprintf("stream_%d", i%2),
									},
								},
							},
							{
								Value: &modelv1.TagValue_Float{
									Float: &modelv1.Float{
										Value: float64(i)*3.14 - 5,
									},
								},
							},
						},
					},
				},
			},
			MessageId: uint64(i + 1),
		}

		if err := writeStream.Send(req); err != nil {
			return fmt.Errorf("failed to send element %d: %w", i, err)
		}
	}

	// 关闭写入流
	if err := writeStream.CloseSend(); err != nil {
		return fmt.Errorf("failed to close write stream: %w", err)
	}

	// 接收响应
	for {
		resp, err := writeStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive response: %w", err)
		}
		fmt.Printf("Write message id: %d written %s\n", resp.GetMessageId(), resp.GetStatus())
	}

	return nil
}

// queryData 查询数据并打印结果
func queryData(ctx context.Context, conn *grpc.ClientConn) error {
	client := streamv1.NewStreamServiceClient(conn)

	req := &streamv1.QueryRequest{
		Groups: []string{group},
		Name:   streamName,
		TimeRange: &modelv1.TimeRange{
			Begin: timestamppb.New(time.Now().Add(-1 * time.Hour)),
			End:   timestamppb.New(time.Now().Add(1 * time.Hour)),
		},
		Projection: &modelv1.TagProjection{
			TagFamilies: []*modelv1.TagProjection_TagFamily{
				{
					Name: "searchable",
					Tags: []string{"stream_id", "trace_id", "value"},
				},
			},
		},
	}

	resp, err := client.Query(ctx, req)
	if err != nil {
		return err
	}

	fmt.Printf("\nQuery Results (%d elements):\n", len(resp.Elements))
	fmt.Println("----------------------------------------")
	for i, elem := range resp.Elements {
		fmt.Printf("Element %d (ID: %s):\n", i+1, elem.ElementId)
		for _, tf := range elem.TagFamilies {
			fmt.Printf("  TagFamily: %s\n", tf.Name)
			for _, tag := range tf.Tags {
				fmt.Printf("    %s: ", tag.Key)
				switch v := tag.Value.Value.(type) {
				case *modelv1.TagValue_Str:
					fmt.Printf("%s (string)\n", v.Str.Value)
				case *modelv1.TagValue_Int:
					fmt.Printf("%d (int)\n", v.Int.Value)
				//case *modelv1.TagValue_Float:
				//	fmt.Printf("%f (float64)\n", v.Float.Value)
				case *modelv1.TagValue_BinaryData:
					fmt.Printf("<binary data>\n")
				default:
					fmt.Printf("<unknown type>\n")
				}
			}
		}
		fmt.Println()
	}

	return nil
}
