// Copyright 2019-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubetest

import (
	"fmt"
	"github.com/onosproject/onos-test/pkg/kube"
	"os"
	"reflect"
	"regexp"
	"runtime/debug"
	"testing"
)

// BenchmarkingSuite is a suite of benchmarks
type BenchmarkingSuite interface{}

// BenchmarkSuite is an identifier interface for benchmark suites
type BenchmarkSuite struct {
	kube kube.API
}

// API returns the Kubernetes API
func (s *BenchmarkSuite) API() kube.API {
	if s.kube == nil {
		s.kube = kube.GetAPIFromEnv()
	}
	return s.kube
}

// SetupBenchmarkSuite is an interface for setting up a suite of benchmarks
type SetupBenchmarkSuite interface {
	SetupBenchmarkSuite()
}

// SetupBenchmark is an interface for setting up individual benchmarks
type SetupBenchmark interface {
	SetupBenchmark()
}

// TearDownBenchmarkSuite is an interface for tearing down a suite of benchmarks
type TearDownBenchmarkSuite interface {
	TearDownBenchmarkSuite()
}

// TearDownBenchmark is an interface for tearing down individual benchmarks
type TearDownBenchmark interface {
	TearDownBenchmark()
}

// BeforeBenchmark is an interface for executing code before every benchmark
type BeforeBenchmark interface {
	BeforeBenchmark(testName string)
}

// AfterBenchmark is an interface for executing code after every benchmark
type AfterBenchmark interface {
	AfterBenchmark(testName string)
}

func failBenchmarkOnPanic(b *testing.B) {
	r := recover()
	if r != nil {
		b.Errorf("test panicked: %v\n%s", r, debug.Stack())
		b.FailNow()
	}
}

// RunBenchmarks runs a benchmark suite
func RunBenchmarks(b *testing.B, suite BenchmarkingSuite) {
	defer failBenchmarkOnPanic(b)

	suiteSetupDone := false

	methodFinder := reflect.TypeOf(suite)
	benchmarks := []testing.InternalBenchmark{}
	for index := 0; index < methodFinder.NumMethod(); index++ {
		method := methodFinder.Method(index)
		ok, err := benchmarkFilter(method.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid regexp for -m: %s\n", err)
			os.Exit(1)
		}
		if !ok {
			continue
		}
		if !suiteSetupDone {
			if setupBenchmarkSuite, ok := suite.(SetupBenchmarkSuite); ok {
				setupBenchmarkSuite.SetupBenchmarkSuite()
			}
			defer func() {
				if tearDownBenchmarkSuite, ok := suite.(TearDownBenchmarkSuite); ok {
					tearDownBenchmarkSuite.TearDownBenchmarkSuite()
				}
			}()
			suiteSetupDone = true
		}
		benchmark := testing.InternalBenchmark{
			Name: method.Name,
			F: func(b *testing.B) {
				defer failBenchmarkOnPanic(b)

				if setupBenchmarkSuite, ok := suite.(SetupBenchmark); ok {
					setupBenchmarkSuite.SetupBenchmark()
				}
				if beforeBenchmarkSuite, ok := suite.(BeforeBenchmark); ok {
					beforeBenchmarkSuite.BeforeBenchmark(method.Name)
				}
				defer func() {
					if afterBenchmarkSuite, ok := suite.(AfterBenchmark); ok {
						afterBenchmarkSuite.AfterBenchmark(method.Name)
					}
					if tearDownBenchmarkSuite, ok := suite.(TearDownBenchmark); ok {
						tearDownBenchmarkSuite.TearDownBenchmark()
					}
				}()
				method.Func.Call([]reflect.Value{reflect.ValueOf(suite), reflect.ValueOf(b)})
			},
		}
		benchmarks = append(benchmarks, benchmark)
	}
	runBenchmarks(b, benchmarks)
}

// runBenchmark runs a benchmark
func runBenchmarks(b *testing.B, benchmarks []testing.InternalBenchmark) {
	for _, benchmark := range benchmarks {
		b.Run(benchmark.Name, benchmark.F)
	}
}

// benchmarkFilter filters benchmark method names
func benchmarkFilter(name string) (bool, error) {
	if ok, _ := regexp.MatchString("^Benchmark", name); !ok {
		return false, nil
	}
	return true, nil
}