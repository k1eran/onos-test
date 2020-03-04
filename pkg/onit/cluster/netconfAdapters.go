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

package cluster

import (
	"fmt"
	"github.com/onosproject/onos-test/pkg/util/random"
)

func newNetconfAdapters(cluster *Cluster) *NetconfAdapters {
	return &NetconfAdapters{
		client:  cluster.client,
		cluster: cluster,
	}
}

// NetconfAdapters provides methods for adding and modifying simulators
type NetconfAdapters struct {
	*client
	cluster *Cluster
}

// New returns a new nc-adapter
func (s *NetconfAdapters) New() *NetconfAdapter {
	return newNetconfAdapter(s.cluster, fmt.Sprintf("nc-adapter-%s", random.NewPetName(2)))
}

// Get gets a simulator by name
func (s *NetconfAdapters) Get(name string) *NetconfAdapter {
	return newNetconfAdapter(s.cluster, name)
}

// List lists the simulators in the cluster
func (s *NetconfAdapters) List() []*NetconfAdapter {
	names := s.listServices(getLabels(simulatorType))
	simulators := make([]*NetconfAdapter, len(names))
	for i, name := range names {
		simulators[i] = s.Get(name)
	}
	return simulators
}
