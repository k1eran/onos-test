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

package test

import (
	"bufio"
	"errors"
	"github.com/ghodss/yaml"
	"github.com/onosproject/onos-test/pkg/kube"
	"github.com/onosproject/onos-test/pkg/util/logging"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const namespace = "kube-test"
const configName = "tests"

// Status test status
type Status string

const (
	// StatusRunning running
	StatusRunning Status = "RUNNING"

	// StatusPassed passed
	StatusPassed Status = "PASSED"

	// StatusFailed failed
	StatusFailed Status = "FAILED"
)

// Record contains information about a test run
type Record struct {
	TestID   string
	Args     []string
	Status   Status
	Message  string
	ExitCode int
}

// NewRunner returns a new test runner
func NewRunner(config *Config) (*Runner, error) {
	kubeAPI, err := kube.GetAPI(config.JobID)
	if err != nil {
		return nil, err
	}
	return &Runner{
		client: kubeAPI.Clientset(),
		config: config,
	}, nil
}

// Runner is a test runner
type Runner struct {
	client *kubernetes.Clientset
	config *Config
}

// Run runs the test
func (r *Runner) Run() error {
	if err := r.ensureNamespace(); err != nil {
		return err
	}

	err := r.startTest()
	if err != nil {
		return err
	}

	step := logging.NewStep(r.config.JobID, "Run test")
	step.Start()

	// Get the stream of logs for the pod
	pod, err := r.getPod()
	if err != nil {
		step.Fail(err)
		return err
	} else if pod == nil {
		return errors.New("cannot locate test pod")
	}

	req := r.client.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Follow: true,
	})
	reader, err := req.Stream()
	if err != nil {
		step.Fail(err)
		return err
	}
	defer reader.Close()

	// Stream the logs to stdout
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logging.Print(scanner.Text())
	}

	// Get the exit message and code
	_, status, err := r.getStatus()
	if err != nil {
		step.Fail(err)
		return err
	}

	step.Complete()
	os.Exit(status)
	return nil
}

// ensureNamespace sets up the test namespace
func (r *Runner) ensureNamespace() error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := r.client.CoreV1().Namespaces().Create(ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return r.setupRBAC()
}

// setupRBAC sets up role based access controls for the cluster
func (r *Runner) setupRBAC() error {
	if err := r.createClusterRole(); err != nil {
		return err
	}
	if err := r.createClusterRoleBinding(); err != nil {
		return err
	}
	if err := r.createServiceAccount(); err != nil {
		return err
	}
	return nil
}

// createClusterRole creates the ClusterRole required by the Atomix controller and tests if not yet created
func (r *Runner) createClusterRole() error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"namespaces",
					"pods",
					"pods/log",
					"pods/exec",
					"services",
					"endpoints",
					"persistentvolumeclaims",
					"events",
					"configmaps",
					"secrets",
					"serviceaccounts",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"apps",
				},
				Resources: []string{
					"deployments",
					"daemonsets",
					"replicasets",
					"statefulsets",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"policy",
				},
				Resources: []string{
					"poddisruptionbudgets",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"batch",
				},
				Resources: []string{
					"jobs",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"rbac.authorization.k8s.io",
				},
				Resources: []string{
					"clusterroles",
					"clusterrolebindings",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"k8s.atomix.io",
				},
				Resources: []string{
					"*",
				},
				Verbs: []string{
					"*",
				},
			},
		},
	}

	_, err := r.client.RbacV1().ClusterRoles().Create(role)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// createClusterRoleBinding creates the ClusterRoleBinding required by the test manager
func (r *Runner) createClusterRoleBinding() error {
	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      namespace,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     namespace,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err := r.client.RbacV1().ClusterRoleBindings().Create(roleBinding)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// createServiceAccount creates a ServiceAccount used by the test manager
func (r *Runner) createServiceAccount() error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: namespace,
		},
	}
	_, err := r.client.CoreV1().ServiceAccounts(namespace).Create(serviceAccount)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// startTests starts running a test job
func (r *Runner) startTest() error {
	step := logging.NewStep(r.config.JobID, "Starting test")
	step.Start()
	if err := r.createTestConfig(); err != nil {
		step.Fail(err)
		return err
	}
	if err := r.createTestJob(); err != nil {
		step.Fail(err)
		return err
	}
	if err := r.awaitTestJobRunning(); err != nil {
		step.Fail(err)
		return err
	}
	step.Complete()
	return nil
}

// createTestConfig creates a ConfigMap for the test configuration
func (r *Runner) createTestConfig() error {
	data, err := yaml.Marshal(r.config)
	if err != nil {
		return err
	}

	cm, err := r.client.CoreV1().ConfigMaps(namespace).Get(configName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: namespace,
			},
			Data: map[string]string{},
		}
		cm, err = r.client.CoreV1().ConfigMaps(namespace).Create(cm)
		if err != nil {
			return err
		}
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[r.config.JobID] = string(data)

	_, err = r.client.CoreV1().ConfigMaps(namespace).Update(cm)
	return err
}

// createTestJob creates the job to run tests
func (r *Runner) createTestJob() error {
	step := logging.NewStep(r.config.JobID, "Deploy test coordinator")
	step.Start()

	zero := int32(0)
	one := int32(1)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.config.JobID,
			Namespace: namespace,
			Annotations: map[string]string{
				"test-id": r.config.JobID,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &one,
			Completions:  &one,
			BackoffLimit: &zero,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": r.config.JobID,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: namespace,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "test",
							Image:           r.config.Image,
							ImagePullPolicy: r.config.PullPolicy,
							Env: []corev1.EnvVar{
								{
									Name:  testContextEnv,
									Value: string(testContextCoordinator),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: filepath.Join(configPath, configFile),
									SubPath:   r.config.JobID,
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
										Name: configName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if r.config.Timeout > 0 {
		timeoutSeconds := int64(r.config.Timeout / time.Second)
		job.Spec.ActiveDeadlineSeconds = &timeoutSeconds
	}

	_, err := r.client.BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		step.Fail(err)
		return err
	}
	step.Complete()
	return nil
}

// awaitTestJobRunning blocks until the test job creates a pod in the RUNNING state
func (r *Runner) awaitTestJobRunning() error {
	for {
		pod, err := r.getPod()
		if err != nil {
			return err
		} else if pod != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// getStatus gets the status message and exit code of the given pod
func (r *Runner) getStatus() (string, int, error) {
	for {
		pod, err := r.getPod()
		if err != nil {
			return "", 0, err
		} else if pod == nil {
			return "", 0, errors.New("cannot locate test pod")
		}
		state := pod.Status.ContainerStatuses[0].State
		if state.Terminated != nil {
			return state.Terminated.Message, int(state.Terminated.ExitCode), nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// getPod finds the Pod for the given test
func (r *Runner) getPod() (*corev1.Pod, error) {
	pods, err := r.client.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: "test=" + r.config.JobID,
	})
	if err != nil {
		return nil, err
	} else if len(pods.Items) > 0 {
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning && len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready {
				return &pod, nil
			}
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				return &pod, nil
			}
		}
	}
	return nil, nil
}

// GetHistory returns the history of test runs on the cluster
func (r *Runner) GetHistory() ([]Record, error) {
	jobs, err := r.client.BatchV1().Jobs(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	records := make([]Record, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		record, err := r.getRecord(job)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// GetRecord returns a single record for the given test
func (r *Runner) GetRecord() (Record, error) {
	job, err := r.client.BatchV1().Jobs(namespace).Get(r.config.JobID, metav1.GetOptions{})
	if err != nil {
		return Record{}, err
	}
	return r.getRecord(*job)
}

// GetRecord returns a single record for the given test
func (r *Runner) getRecord(job batchv1.Job) (Record, error) {
	testID := job.Labels["test"]

	var args []string
	testArgs, ok := job.Annotations["test-args"]
	if ok {
		args = strings.Split(testArgs, ",")
	} else {
		args = make([]string, 0)
	}

	pod, err := r.getPod()
	if err != nil {
		return Record{}, nil
	}

	record := Record{
		TestID: testID,
		Args:   args,
	}

	state := pod.Status.ContainerStatuses[0].State
	if state.Terminated != nil {
		record.Message = state.Terminated.Message
		record.ExitCode = int(state.Terminated.ExitCode)
		if record.ExitCode == 0 {
			record.Status = StatusPassed
		} else {
			record.Status = StatusFailed
		}
	} else {
		record.Status = StatusRunning
	}

	return record, nil
}
