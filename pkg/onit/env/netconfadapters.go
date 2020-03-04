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

package env

import (
	"github.com/onosproject/onos-test/pkg/onit/cluster"
	"sync"
)

const netconfAdapterQueueSize = 10

// NetconfAdaptersSetup provides a setup configuration for multiple netconfAdapters
type NetconfAdaptersSetup interface {
	// With adds a netconfAdapter setup
	With(setups ...NetconfAdapterSetup) NetconfAdaptersSetup

	// AddAll deploys the netconfAdapters in the cluster
	AddAll() ([]NetconfAdapterEnv, error)

	// AddAllOrDie deploys the netconfAdapters and panics if the deployment fails
	AddAllOrDie() []NetconfAdapterEnv
}

var _ NetconfAdaptersSetup = &clusterNetconfAdaptersSetup{}

// clusterNetconfAdaptersSetup is an implementation of the NetconfAdaptersSetup interface
type clusterNetconfAdaptersSetup struct {
	netconfAdapters *cluster.NetconfAdapters
	setups          []NetconfAdapterSetup
}

func (s *clusterNetconfAdaptersSetup) With(setups ...NetconfAdapterSetup) NetconfAdaptersSetup {
	s.setups = append(s.setups, setups...)
	return s
}

func (s *clusterNetconfAdaptersSetup) AddAll() ([]NetconfAdapterEnv, error) {
	wg := &sync.WaitGroup{}
	wg.Add(len(s.setups))

	envCh := make(chan NetconfAdapterEnv, len(s.setups))
	errCh := make(chan error)

	setupQueue := make(chan NetconfAdapterSetup, netconfAdapterQueueSize)
	for i := 0; i < netconfAdapterQueueSize; i++ {
		go func() {
			for setup := range setupQueue {
				if netconfAdapter, err := setup.Add(); err != nil {
					errCh <- err
				} else {
					envCh <- netconfAdapter
				}
				wg.Done()
			}
		}()
	}

	go func() {
		for _, setup := range s.setups {
			setupQueue <- setup
		}
		close(setupQueue)
	}()

	go func() {
		wg.Wait()
		close(envCh)
		close(errCh)
	}()

	for err := range errCh {
		return nil, err
	}

	netconfAdapters := make([]NetconfAdapterEnv, 0, len(s.setups))
	for netconfAdapter := range envCh {
		netconfAdapters = append(netconfAdapters, netconfAdapter)
	}
	return netconfAdapters, nil
}

func (s *clusterNetconfAdaptersSetup) AddAllOrDie() []NetconfAdapterEnv {
	if netconfAdapters, err := s.AddAll(); err != nil {
		panic(err)
	} else {
		return netconfAdapters
	}
}

// NetconfAdaptersEnv provides the netconfAdapters environment
type NetconfAdaptersEnv interface {
	// List returns a list of netconfAdapters in the environment
	List() []NetconfAdapterEnv

	// Get returns the environment for a netconfAdapter service by name
	Get(name string) NetconfAdapterEnv

	// New adds a new netconfAdapter to the environment
	New() NetconfAdapterSetup
}

var _ NetconfAdaptersEnv = &clusterNetconfAdaptersEnv{}

// clusterNetconfAdaptersEnv is an implementation of the NetconfAdapters interface
type clusterNetconfAdaptersEnv struct {
	netconfAdapters *cluster.NetconfAdapters
}

func (e *clusterNetconfAdaptersEnv) List() []NetconfAdapterEnv {
	clusterNetconfAdapters := e.netconfAdapters.List()
	netconfAdapters := make([]NetconfAdapterEnv, len(clusterNetconfAdapters))
	for i, netconfAdapter := range clusterNetconfAdapters {
		netconfAdapters[i] = e.Get(netconfAdapter.Name())
	}
	return netconfAdapters
}

func (e *clusterNetconfAdaptersEnv) Get(name string) NetconfAdapterEnv {
	netconfAdapter := e.netconfAdapters.Get(name)
	return &clusterNetconfAdapterEnv{
		clusterNodeEnv: &clusterNodeEnv{
			node: netconfAdapter.Node,
		},
		netconfAdapter: netconfAdapter,
	}
}

func (e *clusterNetconfAdaptersEnv) New() NetconfAdapterSetup {
	return &clusterNetconfAdapterSetup{
		netconfAdapter: e.netconfAdapters.New(),
	}
}
