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
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/apache/skywalking-banyandb/banyand/metadata/schema"
	"github.com/apache/skywalking-banyandb/pkg/grpchelper"
	"github.com/apache/skywalking-banyandb/pkg/logger"
	"github.com/apache/skywalking-banyandb/pkg/test"
	testflags "github.com/apache/skywalking-banyandb/pkg/test/flags"
	test_measure "github.com/apache/skywalking-banyandb/pkg/test/measure"
	"github.com/apache/skywalking-banyandb/pkg/test/setup"
)

func TestTopNBenchmark(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TopN Benchmark Suite")
}

type suiteConfig struct {
	DistributedAddr string `json:"distributed_addr"`
	BaseTimeNano    int64  `json:"base_time_nano"`
	DataNodeCount   int    `json:"data_node_count"`
}

var (
	deferFunc       func()
	distributedConn *grpc.ClientConn
	baseTime        time.Time
	dataNodeCount   = 3
)

var _ = SynchronizedBeforeSuite(func() []byte {
	Expect(logger.Init(logger.Logging{
		Env:   "dev",
		Level: testflags.LogLevel,
	})).To(Succeed())

	ns := time.Now().Truncate(time.Minute).UnixNano()
	baseTime = time.Unix(0, ns)

	// --- Distributed environment ---
	By("Starting distributed environment")
	tmpDir, tmpDirCleanup, tmpErr := test.NewSpace()
	Expect(tmpErr).NotTo(HaveOccurred())
	dfWriter := setup.NewDiscoveryFileWriter(tmpDir)
	clusterConfig := setup.PropertyClusterConfig(dfWriter)

	By("Starting data nodes")
	var dataClosers []func()
	for i := 0; i < dataNodeCount; i++ {
		nodeDir, nodeDirCleanup, nodeErr := test.NewSpace()
		Expect(nodeErr).NotTo(HaveOccurred())
		_, _, closeDataNode := setup.DataNodeFromDataDir(clusterConfig, nodeDir, "--node-labels", "role=data")
		dataClosers = append(dataClosers, func() {
			closeDataNode()
			nodeDirCleanup()
		})
	}

	By("Loading schema via property")
	setup.PreloadSchemaViaProperty(clusterConfig, test_measure.PreloadSchema)
	clusterConfig.AddLoadedKinds(schema.KindMeasure)

	By("Starting liaison node")
	distributedAddr, closeLiaison := setup.LiaisonNode(clusterConfig, "--data-node-selector", "role=data")

	distributedConnClient, distributedConnErr := grpchelper.Conn(distributedAddr, 10*time.Second,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	Expect(distributedConnErr).NotTo(HaveOccurred())

	By("Writing data to distributed cluster")
	writeTopNData(distributedConnClient, baseTime)

	By("Waiting for data to be queryable")
	time.Sleep(5 * time.Second)

	deferFunc = func() {
		closeLiaison()
		for _, closeFn := range dataClosers {
			closeFn()
		}
		tmpDirCleanup()
	}

	cfg := suiteConfig{
		DistributedAddr: distributedAddr,
		BaseTimeNano:    baseTime.UnixNano(),
		DataNodeCount:   dataNodeCount,
	}
	data, marshalErr := json.Marshal(cfg)
	Expect(marshalErr).NotTo(HaveOccurred())

	distributedConn = distributedConnClient

	return data
}, func(data []byte) {
	var cfg suiteConfig
	Expect(json.Unmarshal(data, &cfg)).To(Succeed())
	baseTime = time.Unix(0, cfg.BaseTimeNano)
	dataNodeCount = cfg.DataNodeCount

	var distributedConnErr error
	distributedConn, distributedConnErr = grpchelper.Conn(cfg.DistributedAddr, 10*time.Second,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	Expect(distributedConnErr).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	if distributedConn != nil {
		Expect(distributedConn.Close()).To(Succeed())
	}
}, func() {})

var _ = ReportAfterSuite("TopN Benchmark Suite", func(_ Report) {
	if deferFunc != nil {
		deferFunc()
	}
})
