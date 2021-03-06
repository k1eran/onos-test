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
	"github.com/atomix/go-client/pkg/client"
	"github.com/onosproject/onos-test/pkg/onit/cluster"
)

// DatabaseEnv is an Atomix database
type DatabaseEnv interface {
	// List returns a list of partitions in the database
	List() []PartitionEnv

	// Get returns the environment for a partition service by name
	Get(name string) PartitionEnv

	// Connect connects to the partition group
	Connect() (*client.Database, error)
}

// clusterDatabaseEnv is an implementation of the Database interface
type clusterDatabaseEnv struct {
	database *cluster.Database
}

func (e *clusterDatabaseEnv) List() []PartitionEnv {
	clusterPartitions := e.database.Partitions()
	partitions := make([]PartitionEnv, len(clusterPartitions))
	for i, partition := range clusterPartitions {
		partitions[i] = &clusterPartitionEnv{
			clusterDeploymentEnv: &clusterDeploymentEnv{
				deployment: partition.Deployment,
			},
		}
	}
	return partitions
}

func (e *clusterDatabaseEnv) Get(name string) PartitionEnv {
	return &clusterPartitionEnv{
		clusterDeploymentEnv: &clusterDeploymentEnv{
			deployment: e.database.Partition(name).Deployment,
		},
	}
}

func (e *clusterDatabaseEnv) Connect() (*client.Database, error) {
	return e.database.Connect()
}
