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

package convert

import (
	"bytes"
	"fmt"
	"math"
	"testing"
)

func TestInt64ToBytes(t *testing.T) {
	testCases := []struct {
		expected []byte
		input    int64
	}{
		{[]byte{127, 255, 255, 255, 255, 255, 255, 156}, -100},
		{[]byte{127, 255, 255, 255, 255, 255, 255, 254}, -2},
		{[]byte{127, 255, 255, 255, 255, 255, 255, 255}, -1},
		{[]byte{128, 0, 0, 0, 0, 0, 0, 0}, 0},
		{[]byte{128, 0, 0, 0, 0, 0, 0, 1}, 1},
		{[]byte{128, 0, 0, 0, 0, 0, 0, 2}, 2},
		{[]byte{128, 0, 0, 0, 0, 0, 0, 100}, 100},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Int64ToBytes(%d)", tc.input), func(t *testing.T) {
			result := Int64ToBytes(tc.input)
			if !bytes.Equal(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestBoolToBytes(t *testing.T) {
	testCases := []struct {
		expected []byte
		input    bool
	}{
		{[]byte{1}, true},
		{[]byte{0}, false},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("BoolToBytes(%t)", tc.input), func(t *testing.T) {
			result := BoolToBytes(tc.input)
			if !bytes.Equal(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestFloat64OrderPreserving(t *testing.T) {
	testCases := []struct {
		name string
		f1   float64
		f2   float64
	}{
		{"negative vs negative", -100.0, -1.0},
		{"negative vs zero", -1.0, 0.0},
		{"negative vs positive", -1.0, 1.0},
		{"zero vs positive", 0.0, 1.0},
		{"positive vs positive", 1.0, 100.0},
		{"small negative vs large negative", -1000.0, -1.0},
		{"small positive vs large positive", 1.0, 1000.0},
		{"infinity", math.Inf(-1), math.Inf(1)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b1 := Float64ToOrderedBytes(tc.f1)
			b2 := Float64ToOrderedBytes(tc.f2)

			cmp := bytes.Compare(b1, b2)
			if cmp >= 0 {
				t.Errorf("Expected bytes.Compare(%v, %v) < 0, got %d", tc.f1, tc.f2, cmp)
			}
		})
	}
}

func TestFloat64RoundTrip(t *testing.T) {
	testCases := []float64{
		0.0,
		1.0,
		-1.0,
		123.456,
		-123.456,
		0.000001,
		-0.000001,
		1e10,
		-1e10,
		1e-10,
		-1e-10,
		math.Inf(1),
		math.Inf(-1),
		math.MaxFloat64,
		-math.MaxFloat64,
		math.SmallestNonzeroFloat64,
		-math.SmallestNonzeroFloat64,
		math.NaN(),
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("roundtrip_%v", tc), func(t *testing.T) {
			encoded := Float64ToOrderedBytes(tc)
			decoded := OrderedBytesToFloat64(encoded)

			if math.IsNaN(tc) {
				if !math.IsNaN(decoded) {
					t.Errorf("Round trip failed: input=%v, decoded=%v", tc, decoded)
				}
			} else if decoded != tc {
				t.Errorf("Round trip failed: input=%v, decoded=%v", tc, decoded)
			}
		})
	}
}
