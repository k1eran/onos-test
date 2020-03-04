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
	"context"
	"errors"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/watch"
	"time"

	"github.com/onosproject/onos-test/pkg/util/logging"
	"github.com/onosproject/onos-topo/api/device"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	netconfAdapterType           = "netconfAdapter"
	netconfAdapterLabel          = "netconfAdapter"
	netconfAdapterImage          = "onosproject/gnmi-netconf-adapter:latest"
	netconfAdapterService        = "gnmi-netconf-adapter"
	netconfAdapterDeviceType     = "Junos"
	netconfAdapterDeviceVersion  = "19.3.1.8"
	netconfAdapterSecurePortName = "secure"
	netconfAdapterSecurePort     = 11161
	netconfAdapterGnmiPortEnv    = "GNMI_PORT"
)

/* This is a placeholder for future config for the gnmi netconfAdapter.
 */
const netconfAdapterConfig = `
{
}
`

func newNetconfAdapter(cluster *Cluster, name string) *NetconfAdapter {
	node := newNode(cluster)
	node.SetName(name)
	node.SetImage(netconfAdapterImage)
	node.SetPort(netconfAdapterSecurePort)
	return &NetconfAdapter{
		Node:          node,
		add:           true,
		deviceType:    netconfAdapterDeviceType,
		deviceVersion: netconfAdapterDeviceVersion,
	}
}

// NetconfAdapter provides methods for adding and modifying netconfAdapters
type NetconfAdapter struct {
	*Node
	add           bool
	deviceType    string
	deviceVersion string
	deviceTimeout *time.Duration
}

// AddDevice returns whether to add the device to the topo service
func (s *NetconfAdapter) AddDevice() bool {
	return s.add
}

// SetAddDevice sets whether to add the device to the topo service
func (s *NetconfAdapter) SetAddDevice(add bool) {
	s.add = add
}

// DeviceType returns the device type
func (s *NetconfAdapter) DeviceType() string {
	return s.deviceType
}

// SetDeviceType sets the device type
func (s *NetconfAdapter) SetDeviceType(deviceType string) {
	s.deviceType = deviceType
}

// DeviceVersion returns the device version
func (s *NetconfAdapter) DeviceVersion() string {
	return s.deviceVersion
}

// SetDeviceVersion sets the device version
func (s *NetconfAdapter) SetDeviceVersion(version string) {
	s.deviceVersion = version
}

// DeviceTimeout returns the device timeout
func (s *NetconfAdapter) DeviceTimeout() *time.Duration {
	return s.deviceTimeout
}

// SetDeviceTimeout sets the device timeout
func (s *NetconfAdapter) SetDeviceTimeout(timeout time.Duration) {
	s.deviceTimeout = &timeout
}

// Setup adds the netconfAdapter to the cluster
func (s *NetconfAdapter) Setup() error {
	step := logging.NewStep(s.namespace, fmt.Sprintf("Add netconfAdapter %s", s.Name()))
	step.Start()
	step.Logf("Creating %s ConfigMap", s.Name())
	if err := s.createConfigMap(); err != nil {
		step.Fail(err)
		return err
	}
	step.Logf("Creating %s Pod", s.Name())
	if err := s.createPod(); err != nil {
		step.Fail(err)
		return err
	}
	step.Logf("Creating %s Service", s.Name())
	if err := s.createService(); err != nil {
		step.Fail(err)
		return err
	}
	step.Logf("Waiting for %s to become ready", s.Name())
	if err := s.awaitReady(); err != nil {
		step.Fail(err)
		return err
	}
	if s.add {
		step.Logf("Adding %s to onos-topo", s.Name())
		if err := s.addDevice(); err != nil {
			step.Fail(err)
			return err
		}
	}
	step.Complete()
	return nil
}

// getLabels gets the netconfAdapter labels
func (s *NetconfAdapter) getLabels() map[string]string {
	labels := getLabels(netconfAdapterType)
	labels[netconfAdapterLabel] = s.name
	return labels
}

// createConfigMap creates a netconfAdapter configuration
func (s *NetconfAdapter) createConfigMap() error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    s.getLabels(),
		},
		Data: map[string]string{
			"config.json": netconfAdapterConfig,
		},
	}
	_, err := s.kubeClient.CoreV1().ConfigMaps(s.namespace).Create(cm)
	return err
}

// createPod creates a netconfAdapter pod
func (s *NetconfAdapter) createPod() error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    s.getLabels(),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            netconfAdapterService,
					Image:           s.image,
					ImagePullPolicy: s.pullPolicy,
					Env: []corev1.EnvVar{
						{
							Name:  netconfAdapterGnmiPortEnv,
							Value: fmt.Sprintf("%d", netconfAdapterSecurePort),
						},
						//{
						//	Name:  netconfAdapterGnmiInsecurePortEnv,
						//	Value: fmt.Sprintf("%d", s.Port()),
						//},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          netconfAdapterSecurePortName,
							ContainerPort: netconfAdapterSecurePort,
						},
						//{
						//	Name:          netconfAdapterInsecurePortName,
						//	ContainerPort: int32(s.Port()),
						//},
					},
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(s.Port()),
							},
						},
						PeriodSeconds:    1,
						FailureThreshold: 30,
					},
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(s.Port()),
							},
						},
						InitialDelaySeconds: 15,
						PeriodSeconds:       20,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/etc/netconfAdapter/configs",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: s.name,
							},
						},
					},
				},
			},
		},
	}

	_, err := s.kubeClient.CoreV1().Pods(s.namespace).Create(pod)
	return err
}

// createService creates a netconfAdapter service
func (s *NetconfAdapter) createService() error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
			Labels:    s.getLabels(),
		},
		Spec: corev1.ServiceSpec{
			Selector: s.getLabels(),
			Ports: []corev1.ServicePort{
				{
					Name: netconfAdapterSecurePortName,
					Port: netconfAdapterSecurePort,
				},
				//{
				//	Name: netconfAdapterInsecurePortName,
				//	Port: int32(s.Port()),
				//},
			},
		},
	}

	_, err := s.kubeClient.CoreV1().Services(s.namespace).Create(service)
	return err
}

// awaitReady waits for the given netconfAdapter to complete startup
func (s *NetconfAdapter) awaitReady() error {
	for {
		pod, err := s.kubeClient.CoreV1().Pods(s.namespace).Get(s.name, metav1.GetOptions{})
		if err != nil {
			return err
		} else if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready {
			return nil
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// connectTopo connects to the topo service
func (s *NetconfAdapter) connectTopo() (*grpc.ClientConn, error) {
	tlsConfig, err := s.Credentials()
	if err != nil {
		return nil, err
	}
	return grpc.Dial(s.cluster.Topo().Address(s.cluster.Topo().Ports()[0].Name), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
}

// addDevice adds the device to the topo service
func (s *NetconfAdapter) addDevice() error {
	tlsConfig, err := s.Credentials()
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(s.cluster.Topo().Address(s.cluster.Topo().Ports()[0].Name), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := device.NewDeviceServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	defer cancel()
	_, err = client.Add(ctx, &device.AddRequest{
		Device: &device.Device{
			ID:      device.ID(s.Name()),
			Address: s.Address(),
			Type:    device.Type(s.DeviceType()),
			Version: s.DeviceVersion(),
			Timeout: s.deviceTimeout,
			TLS: device.TlsConfig{
				// TODO KMP check this okay
				// Plain: true,
				Insecure: true,
			},
		},
	})
	return err
}

// AwaitDevicePredicate waits for the given device predicate
func (s *NetconfAdapter) AwaitDevicePredicate(predicate func(*device.Device) bool, timeout time.Duration) error {
	conn, err := s.connectTopo()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := device.NewDeviceServiceClient(conn)

	// Set a timer within which the device must reach the connected/available state
	errCh := make(chan error)
	timer := time.NewTimer(5 * time.Second)

	// Open a stream to listen for events from the device service
	stream, err := client.List(context.Background(), &device.ListRequest{
		Subscribe: true,
	})
	if err != nil {
		return err
	}

	// Start a goroutine to listen for device events from the topo service
	go func() {
		for {
			response, err := stream.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				errCh <- err
				close(errCh)
				return
			}

			if predicate(response.Device) {
				timer.Stop()
				close(errCh)
				return
			}
		}
	}()

	select {
	// If the timer fires, return a timeout error
	case _, ok := <-timer.C:
		if !ok {
			return errors.New("device predicate timed out")
		}
		return nil
	// If an error is received on the error channel, return the error. Otherwise, return nil
	case err := <-errCh:
		return err
	}
}

// TearDown removes the netconfAdapter from the cluster
func (s *NetconfAdapter) TearDown() error {
	var err error

	if e := s.removeDevice(); e != nil {
		err = e
	}
	if e := s.deletePod(); e != nil {
		err = e
	}
	if e := s.deleteService(); e != nil {
		err = e
	}
	if e := s.deleteConfigMap(); e != nil {
		err = e
	}
	if e := s.waitForDeletePod(); e != nil {
		err = e
	}
	return err
}

// removeDevice removes the device from the topo service
func (s *NetconfAdapter) removeDevice() error {
	if !s.add {
		return nil
	}

	tlsConfig, err := s.Credentials()
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(s.cluster.Topo().Address(s.cluster.Topo().Ports()[0].Name), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := device.NewDeviceServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), topoTimeout)
	response, err := client.Get(ctx, &device.GetRequest{
		ID: device.ID(s.Name()),
	})
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), topoTimeout)
	_, err = client.Remove(ctx, &device.RemoveRequest{
		Device: response.Device,
	})
	cancel()
	return err
}

// deleteConfigMap deletes a netconfAdapter ConfigMap by name
func (s *NetconfAdapter) deleteConfigMap() error {
	return s.kubeClient.CoreV1().ConfigMaps(s.namespace).Delete(s.name, &metav1.DeleteOptions{})
}

// deletePod deletes a netconfAdapter Pod by name
func (s *NetconfAdapter) deletePod() error {
	return s.kubeClient.CoreV1().Pods(s.namespace).Delete(s.name, &metav1.DeleteOptions{})
}

func (s *NetconfAdapter) waitForDeletePod() error {
	w, e := s.kubeClient.CoreV1().Pods(s.client.namespace).Watch(metav1.ListOptions{})
	if e != nil {
		return e
	}
	for event := range w.ResultChan() {
		switch event.Type {
		case watch.Deleted:
			w.Stop()
		}
	}
	return nil
}

// deleteService deletes a netconfAdapter Service by name
func (s *NetconfAdapter) deleteService() error {
	return s.kubeClient.CoreV1().Services(s.namespace).Delete(s.name, &metav1.DeleteOptions{})
}
