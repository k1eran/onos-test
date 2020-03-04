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
	"context"
	"github.com/onosproject/onos-test/pkg/onit/cluster"
	"github.com/openconfig/gnmi/client"
	gnmi "github.com/openconfig/gnmi/client/gnmi"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"time"
)

// NetconfAdapterSetup is an interface for setting up a netconfAdapter
type NetconfAdapterSetup interface {
	// SetName sets the netconfAdapter name
	SetName(name string) NetconfAdapterSetup

	// SetImage sets the image to deploy
	SetImage(image string) NetconfAdapterSetup

	// SetPullPolicy sets the image pull policy
	SetPullPolicy(pullPolicy corev1.PullPolicy) NetconfAdapterSetup

	// SetDeviceType sets the device type
	SetDeviceType(deviceType string) NetconfAdapterSetup

	// SetDeviceVersion sets the device version
	SetDeviceVersion(version string) NetconfAdapterSetup

	// SetDeviceTimeout sets the device timeout
	SetDeviceTimeout(timeout time.Duration) NetconfAdapterSetup

	// SetAddDevice sets whether to add the device to the topo service
	SetAddDevice(add bool) NetconfAdapterSetup

	// Add deploys the netconfAdapter in the cluster
	Add() (NetconfAdapterEnv, error)

	// AddOrDie deploys the netconfAdapter and panics if the deployment fails
	AddOrDie() NetconfAdapterEnv

	// SetPort sets the netconfAdapter port
	SetPort(port int) NetconfAdapterSetup
}

var _ NetconfAdapterSetup = &clusterNetconfAdapterSetup{}

// clusterNetconfAdapterSetup is an implementation of the NetconfAdapterSetup interface
type clusterNetconfAdapterSetup struct {
	netconfAdapter *cluster.NetconfAdapter
}

func (s *clusterNetconfAdapterSetup) SetName(name string) NetconfAdapterSetup {
	s.netconfAdapter.SetName(name)
	return s
}

func (s *clusterNetconfAdapterSetup) SetImage(image string) NetconfAdapterSetup {
	s.netconfAdapter.SetImage(image)
	return s
}

func (s *clusterNetconfAdapterSetup) SetPullPolicy(pullPolicy corev1.PullPolicy) NetconfAdapterSetup {
	s.netconfAdapter.SetPullPolicy(pullPolicy)
	return s
}

func (s *clusterNetconfAdapterSetup) SetDeviceType(deviceType string) NetconfAdapterSetup {
	s.netconfAdapter.SetDeviceType(deviceType)
	return s
}

func (s *clusterNetconfAdapterSetup) SetDeviceVersion(version string) NetconfAdapterSetup {
	s.netconfAdapter.SetDeviceVersion(version)
	return s
}

func (s *clusterNetconfAdapterSetup) SetDeviceTimeout(timeout time.Duration) NetconfAdapterSetup {
	s.netconfAdapter.SetDeviceTimeout(timeout)
	return s
}

func (s *clusterNetconfAdapterSetup) SetAddDevice(addDevice bool) NetconfAdapterSetup {
	s.netconfAdapter.SetAddDevice(addDevice)
	return s
}

func (s *clusterNetconfAdapterSetup) SetPort(port int) NetconfAdapterSetup {
	s.netconfAdapter.SetPort(port)
	return s
}

func (s *clusterNetconfAdapterSetup) Add() (NetconfAdapterEnv, error) {
	if err := s.netconfAdapter.Setup(); err != nil {
		return nil, err
	}
	return &clusterNetconfAdapterEnv{
		clusterNodeEnv: &clusterNodeEnv{
			node: s.netconfAdapter.Node,
		},
		netconfAdapter: s.netconfAdapter,
	}, nil
}

func (s *clusterNetconfAdapterSetup) AddOrDie() NetconfAdapterEnv {
	network, err := s.Add()
	if err != nil {
		panic(err)
	}
	return network
}

// NetconfAdapterEnv provides the environment for a single netconfAdapter
type NetconfAdapterEnv interface {
	NodeEnv

	// Destination returns the gNMI client destination
	Destination() client.Destination

	// NewGNMIClient returns the gNMI client
	NewGNMIClient() (*gnmi.Client, error)

	// Await waits for the netconfAdapter device state to match the given predicate
	Await(predicate DevicePredicate, timeout time.Duration) error

	// Remove removes the netconfAdapter
	Remove() error

	// RemoveOrDie removes the netconfAdapter and panics if the remove fails
	RemoveOrDie()
}

var _ NetconfAdapterEnv = &clusterNetconfAdapterEnv{}

// clusterNetconfAdapterEnv is an implementation of the NetconfAdapter interface
type clusterNetconfAdapterEnv struct {
	*clusterNodeEnv
	netconfAdapter *cluster.NetconfAdapter
}

func (e *clusterNetconfAdapterEnv) Connect() (*grpc.ClientConn, error) {
	return grpc.Dial(e.Address(), grpc.WithInsecure())
}

func (e *clusterNetconfAdapterEnv) Destination() client.Destination {
	return client.Destination{
		Addrs:   []string{e.Address()},
		Target:  e.Name(),
		TLS:     e.Credentials(),
		Timeout: 10 * time.Second,
	}
}

func (e *clusterNetconfAdapterEnv) NewGNMIClient() (*gnmi.Client, error) {
	conn, err := e.Connect()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := gnmi.NewFromConn(ctx, conn, e.Destination())
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (e *clusterNetconfAdapterEnv) Await(predicate DevicePredicate, timeout time.Duration) error {
	return e.netconfAdapter.AwaitDevicePredicate(predicate, timeout)
}

func (e *clusterNetconfAdapterEnv) Remove() error {
	return e.netconfAdapter.TearDown()
}

func (e *clusterNetconfAdapterEnv) RemoveOrDie() {
	if err := e.Remove(); err != nil {
		panic(err)
	}
}
