// Copyright 2022 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stack

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
)

func Test_translateConfigMap(t *testing.T) {
	s := &model.Stack{
		Manifest: []byte("manifest"),
		Name:     "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Image: "image",
			},
		},
	}
	result := translateConfigMap(s)
	if result.Name != "okteto-stackName" {
		t.Errorf("Wrong configmap name: '%s'", result.Name)
	}
	if result.Labels[model.StackLabel] != "true" {
		t.Errorf("Wrong labels: '%s'", result.Labels)
	}
	if result.Data[NameField] != "stackName" {
		t.Errorf("Wrong data.name: '%s'", result.Data[NameField])
	}
	if result.Data[YamlField] != base64.StdEncoding.EncodeToString(s.Manifest) {
		t.Errorf("Wrong data.yaml: '%s'", result.Data[YamlField])
	}
}

func Test_translateDeployment(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				Image:           "image",
				Replicas:        3,
				StopGracePeriod: 20,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []model.EnvVar{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports: []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
			},
		},
	}
	result := translateDeployment("svcName", s)
	if result.Name != "svcName" {
		t.Errorf("Wrong deployment name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackName",
		model.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong deployment labels: '%s'", result.Labels)
	}
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong deployment annotations: '%s'", result.Annotations)
	}
	if *result.Spec.Replicas != 3 {
		t.Errorf("Wrong deployment spec.replicas: '%d'", *result.Spec.Replicas)
	}
	selector := map[string]string{
		model.StackNameLabel:        "stackName",
		model.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Spec.Selector.MatchLabels, selector) {
		t.Errorf("Wrong spec.selector: '%s'", result.Spec.Selector.MatchLabels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Labels, labels) {
		t.Errorf("Wrong spec.template.labels: '%s'", result.Spec.Template.Labels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong deployment spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong deployment container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong deployment container.image: '%s'", c.Image)
	}
	if !reflect.DeepEqual(c.Command, []string{"command1", "command2"}) {
		t.Errorf("Wrong container.command: '%v'", c.Command)
	}
	if !reflect.DeepEqual(c.Args, []string{"args1", "args2"}) {
		t.Errorf("Wrong container.args: '%v'", c.Args)
	}
	env := []apiv1.EnvVar{{Name: "env1", Value: "value1"}, {Name: "env2", Value: "value2"}}
	if !reflect.DeepEqual(c.Env, env) {
		t.Errorf("Wrong container.env: '%v'", c.Env)
	}
	ports := []apiv1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 90}}
	if !reflect.DeepEqual(c.Ports, ports) {
		t.Errorf("Wrong container.ports: '%v'", c.Ports)
	}
	if c.SecurityContext != nil {
		t.Errorf("Wrong deployment container.security_context: '%v'", c.SecurityContext)
	}
	if !reflect.DeepEqual(c.Resources, apiv1.ResourceRequirements{}) {
		t.Errorf("Wrong container.resources: '%v'", c.Resources)
	}

}

func Test_translateStatefulSet(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				Image:           "image",
				Replicas:        3,
				StopGracePeriod: 20,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []model.EnvVar{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports:   []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
				CapAdd:  []apiv1.Capability{apiv1.Capability("CAP_ADD")},
				CapDrop: []apiv1.Capability{apiv1.Capability("CAP_DROP")},

				Volumes: []model.StackVolume{{RemotePath: "/volume1"}, {RemotePath: "/volume2"}},
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("100m")},
						Memory: model.Quantity{Value: resource.MustParse("1Gi")},
					},
					Requests: model.ServiceResources{
						Storage: model.StorageResource{
							Size:  model.Quantity{Value: resource.MustParse("20Gi")},
							Class: "class-name",
						},
					},
				},
			},
		},
	}
	result := translateStatefulSet("svcName", s)
	if result.Name != "svcName" {
		t.Errorf("Wrong statefulset name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackName",
		model.StackServiceNameLabel: "svcName",
	}
	assert.Equal(t, labels, result.Labels)

	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong statefulset annotations: '%s'", result.Annotations)
	}
	if *result.Spec.Replicas != 3 {
		t.Errorf("Wrong statefulset spec.replicas: '%d'", *result.Spec.Replicas)
	}
	selector := map[string]string{
		model.StackNameLabel:        "stackName",
		model.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Spec.Selector.MatchLabels, selector) {
		t.Errorf("Wrong spec.selector: '%s'", result.Spec.Selector.MatchLabels)
	}
	assert.Equal(t, labels, result.Spec.Template.Labels)
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong statefulset spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	initContainer := apiv1.Container{
		Name:    fmt.Sprintf("init-%s", "svcName"),
		Image:   "busybox",
		Command: []string{"sh", "-c", "chmod 777 /data"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/data",
				Name:      pvcName,
			},
		},
	}
	assert.Equal(t, initContainer, result.Spec.Template.Spec.InitContainers[0])
	initVolumeContainer := apiv1.Container{
		Name:            fmt.Sprintf("init-volume-%s", "svcName"),
		Image:           "image",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         []string{"sh", "-c", "echo initializing volume... && (cp -Rv /volume1/. /init-volume-0 || true) && (cp -Rv /volume2/. /init-volume-1 || true)"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/init-volume-0",
				Name:      pvcName,
				SubPath:   "data-0",
			},
			{
				MountPath: "/init-volume-1",
				Name:      pvcName,
				SubPath:   "data-1",
			},
		},
	}
	assert.Equal(t, initVolumeContainer, result.Spec.Template.Spec.InitContainers[1])

	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong statefulset container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong statefulset container.image: '%s'", c.Image)
	}
	if !reflect.DeepEqual(c.Command, []string{"command1", "command2"}) {
		t.Errorf("Wrong container.command: '%v'", c.Command)
	}
	if !reflect.DeepEqual(c.Args, []string{"args1", "args2"}) {
		t.Errorf("Wrong container.args: '%v'", c.Args)
	}
	env := []apiv1.EnvVar{{Name: "env1", Value: "value1"}, {Name: "env2", Value: "value2"}}
	if !reflect.DeepEqual(c.Env, env) {
		t.Errorf("Wrong container.env: '%v'", c.Env)
	}
	ports := []apiv1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 90}}
	if !reflect.DeepEqual(c.Ports, ports) {
		t.Errorf("Wrong container.ports: '%v'", c.Ports)
	}
	securityContext := apiv1.SecurityContext{
		Capabilities: &apiv1.Capabilities{
			Add:  []apiv1.Capability{apiv1.Capability("CAP_ADD")},
			Drop: []apiv1.Capability{apiv1.Capability("CAP_DROP")},
		},
	}
	if !reflect.DeepEqual(*c.SecurityContext, securityContext) {
		t.Errorf("Wrong statefulset container.security_context: '%v'", c.SecurityContext)
	}
	resources := apiv1.ResourceRequirements{
		Limits: apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse("100m"),
			apiv1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	if !reflect.DeepEqual(c.Resources, resources) {
		t.Errorf("Wrong container.resources: '%v'", c.Resources)
	}
	volumeMounts := []apiv1.VolumeMount{
		{
			MountPath: "/volume1",
			Name:      pvcName,
			SubPath:   "data-0",
		},
		{
			MountPath: "/volume2",
			Name:      pvcName,
			SubPath:   "data-1",
		},
	}
	assert.Equal(t, volumeMounts, c.VolumeMounts)

	vct := result.Spec.VolumeClaimTemplates[0]
	if vct.Name != pvcName {
		t.Errorf("Wrong statefulset name: '%s'", vct.Name)
	}
	if !reflect.DeepEqual(vct.Labels, labels) {
		t.Errorf("Wrong statefulset labels: '%s'", vct.Labels)
	}
	if !reflect.DeepEqual(vct.Annotations, annotations) {
		t.Errorf("Wrong statefulset annotations: '%s'", vct.Annotations)
	}
	volumeClaimTemplateSpec := apiv1.PersistentVolumeClaimSpec{
		AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
		Resources: apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				"storage": resource.MustParse("20Gi"),
			},
		},
		StorageClassName: pointer.StringPtr("class-name"),
	}
	if !reflect.DeepEqual(vct.Spec, volumeClaimTemplateSpec) {
		t.Errorf("Wrong statefulset volume claim template: '%v'", vct.Spec)
	}

}

func Test_translateJobWithoutVolumes(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				Image:           "image",
				StopGracePeriod: 20,
				Replicas:        3,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []model.EnvVar{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports:         []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
				CapAdd:        []apiv1.Capability{apiv1.Capability("CAP_ADD")},
				CapDrop:       []apiv1.Capability{apiv1.Capability("CAP_DROP")},
				RestartPolicy: apiv1.RestartPolicyNever,
				BackOffLimit:  5,
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("100m")},
						Memory: model.Quantity{Value: resource.MustParse("1Gi")},
					},
					Requests: model.ServiceResources{
						Storage: model.StorageResource{
							Size:  model.Quantity{Value: resource.MustParse("20Gi")},
							Class: "class-name",
						},
					},
				},
			},
		},
	}
	result := translateJob("svcName", s)
	if result.Name != "svcName" {
		t.Errorf("Wrong job name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackName",
		model.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong job labels: '%s'", result.Labels)
	}
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong job annotations: '%s'", result.Annotations)
	}
	if *result.Spec.Completions != 3 {
		t.Errorf("Wrong job spec.completions: '%d'", *result.Spec.Completions)
	}
	if *result.Spec.Parallelism != 1 {
		t.Errorf("Wrong job spec.parallelism: '%d'", *result.Spec.Parallelism)
	}
	if *result.Spec.BackoffLimit != 5 {
		t.Errorf("Wrong job spec.max_attempts: '%d'", *result.Spec.BackoffLimit)
	}
	if !reflect.DeepEqual(result.Spec.Template.Labels, labels) {
		t.Errorf("Wrong spec.template.labels: '%s'", result.Spec.Template.Labels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong job spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	if len(result.Spec.Template.Spec.InitContainers) > 0 {
		t.Errorf("Wrong job spec.template.spec.initContainers: '%d'", len(result.Spec.Template.Spec.InitContainers))
	}
	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong job container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong job container.image: '%s'", c.Image)
	}
	if !reflect.DeepEqual(c.Command, []string{"command1", "command2"}) {
		t.Errorf("Wrong container.command: '%v'", c.Command)
	}
	if !reflect.DeepEqual(c.Args, []string{"args1", "args2"}) {
		t.Errorf("Wrong container.args: '%v'", c.Args)
	}
	env := []apiv1.EnvVar{{Name: "env1", Value: "value1"}, {Name: "env2", Value: "value2"}}
	if !reflect.DeepEqual(c.Env, env) {
		t.Errorf("Wrong container.env: '%v'", c.Env)
	}
	ports := []apiv1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 90}}
	if !reflect.DeepEqual(c.Ports, ports) {
		t.Errorf("Wrong container.ports: '%v'", c.Ports)
	}
	securityContext := apiv1.SecurityContext{
		Capabilities: &apiv1.Capabilities{
			Add:  []apiv1.Capability{apiv1.Capability("CAP_ADD")},
			Drop: []apiv1.Capability{apiv1.Capability("CAP_DROP")},
		},
	}
	if !reflect.DeepEqual(*c.SecurityContext, securityContext) {
		t.Errorf("Wrong job container.security_context: '%v'", c.SecurityContext)
	}
	resources := apiv1.ResourceRequirements{
		Limits: apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse("100m"),
			apiv1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	if !reflect.DeepEqual(c.Resources, resources) {
		t.Errorf("Wrong container.resources: '%v'", c.Resources)
	}
	if len(c.VolumeMounts) > 0 {
		t.Errorf("Wrong c.VolumeMounts: '%d'", len(c.VolumeMounts))
	}
}

func Test_translateJobWithVolumes(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				Image:           "image",
				StopGracePeriod: 20,
				Replicas:        3,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []model.EnvVar{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports:         []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
				CapAdd:        []apiv1.Capability{apiv1.Capability("CAP_ADD")},
				CapDrop:       []apiv1.Capability{apiv1.Capability("CAP_DROP")},
				RestartPolicy: apiv1.RestartPolicyNever,
				BackOffLimit:  5,
				Volumes:       []model.StackVolume{{RemotePath: "/volume1"}, {RemotePath: "/volume2"}},
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("100m")},
						Memory: model.Quantity{Value: resource.MustParse("1Gi")},
					},
					Requests: model.ServiceResources{
						Storage: model.StorageResource{
							Size:  model.Quantity{Value: resource.MustParse("20Gi")},
							Class: "class-name",
						},
					},
				},
			},
		},
	}
	result := translateJob("svcName", s)
	if result.Name != "svcName" {
		t.Errorf("Wrong job name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackName",
		model.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong job labels: '%s'", result.Labels)
	}
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong job annotations: '%s'", result.Annotations)
	}
	if *result.Spec.Completions != 3 {
		t.Errorf("Wrong job spec.completions: '%d'", *result.Spec.Completions)
	}
	if *result.Spec.Parallelism != 1 {
		t.Errorf("Wrong job spec.parallelism: '%d'", *result.Spec.Parallelism)
	}
	if *result.Spec.BackoffLimit != 5 {
		t.Errorf("Wrong job spec.max_attempts: '%d'", *result.Spec.BackoffLimit)
	}
	if !reflect.DeepEqual(result.Spec.Template.Labels, labels) {
		t.Errorf("Wrong spec.template.labels: '%s'", result.Spec.Template.Labels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong job spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	initContainer := apiv1.Container{
		Name:    fmt.Sprintf("init-%s", "svcName"),
		Image:   "busybox",
		Command: []string{"sh", "-c", "chmod 777 /data"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/data",
				Name:      pvcName,
			},
		},
	}
	if !reflect.DeepEqual(result.Spec.Template.Spec.InitContainers[0], initContainer) {
		t.Errorf("Wrong job init container: '%v' but expected '%v'", result.Spec.Template.Spec.InitContainers[0], initContainer)
	}
	initVolumeContainer := apiv1.Container{
		Name:            fmt.Sprintf("init-volume-%s", "svcName"),
		Image:           "image",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         []string{"sh", "-c", "echo initializing volume... && (cp -Rv /volume1/. /init-volume-0 || true) && (cp -Rv /volume2/. /init-volume-1 || true)"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/init-volume-0",
				Name:      pvcName,
				SubPath:   "data-0",
			},
			{
				MountPath: "/init-volume-1",
				Name:      pvcName,
				SubPath:   "data-1",
			},
		},
	}
	if !reflect.DeepEqual(result.Spec.Template.Spec.InitContainers[1], initVolumeContainer) {
		t.Errorf("Wrong job init container: '%v' but expected '%v'", result.Spec.Template.Spec.InitContainers[1], initVolumeContainer)
	}
	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong job container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong job container.image: '%s'", c.Image)
	}
	if !reflect.DeepEqual(c.Command, []string{"command1", "command2"}) {
		t.Errorf("Wrong container.command: '%v'", c.Command)
	}
	if !reflect.DeepEqual(c.Args, []string{"args1", "args2"}) {
		t.Errorf("Wrong container.args: '%v'", c.Args)
	}
	env := []apiv1.EnvVar{{Name: "env1", Value: "value1"}, {Name: "env2", Value: "value2"}}
	if !reflect.DeepEqual(c.Env, env) {
		t.Errorf("Wrong container.env: '%v'", c.Env)
	}
	ports := []apiv1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 90}}
	if !reflect.DeepEqual(c.Ports, ports) {
		t.Errorf("Wrong container.ports: '%v'", c.Ports)
	}
	securityContext := apiv1.SecurityContext{
		Capabilities: &apiv1.Capabilities{
			Add:  []apiv1.Capability{apiv1.Capability("CAP_ADD")},
			Drop: []apiv1.Capability{apiv1.Capability("CAP_DROP")},
		},
	}
	if !reflect.DeepEqual(*c.SecurityContext, securityContext) {
		t.Errorf("Wrong job container.security_context: '%v'", c.SecurityContext)
	}
	resources := apiv1.ResourceRequirements{
		Limits: apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse("100m"),
			apiv1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	if !reflect.DeepEqual(c.Resources, resources) {
		t.Errorf("Wrong container.resources: '%v'", c.Resources)
	}
	volumeMounts := []apiv1.VolumeMount{
		{
			MountPath: "/volume1",
			Name:      pvcName,
			SubPath:   "data-0",
		},
		{
			MountPath: "/volume2",
			Name:      pvcName,
			SubPath:   "data-1",
		},
	}
	if !reflect.DeepEqual(c.VolumeMounts, volumeMounts) {
		t.Errorf("Wrong container.volume_mounts: '%v'", c.VolumeMounts)
	}
}

func Test_translateService(t *testing.T) {

	var tests = []struct {
		name     string
		stack    *model.Stack
		expected *apiv1.Service
	}{
		{
			name: "translate svc no public endpoints",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1": "value1",
							"annotation2": "value2",
						},
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							},
						},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Annotations: map[string]string{
						"annotation1": "value1",
						"annotation2": "value2",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc public endpoints",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},

						Public: true,
						Annotations: model.Annotations{
							"annotation1":                     "value1",
							"annotation2":                     "value2",
							model.OktetoAutoIngressAnnotation: "true",
						},
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							},
						},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Annotations: map[string]string{
						"annotation1":                     "value1",
						"annotation2":                     "value2",
						model.OktetoAutoIngressAnnotation: "true",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc private endpoints",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1":                     "value1",
							"annotation2":                     "value2",
							model.OktetoAutoIngressAnnotation: "private",
						},
						Public: true,
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							}},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Annotations: map[string]string{
						"annotation1":                     "value1",
						"annotation2":                     "value2",
						model.OktetoAutoIngressAnnotation: "private",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc private endpoints by private annotation",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1":                    "value1",
							"annotation2":                    "value2",
							model.OktetoPrivateSvcAnnotation: "true",
						},
						Public: true,
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							}},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Annotations: map[string]string{
						"annotation1":                    "value1",
						"annotation2":                    "value2",
						model.OktetoPrivateSvcAnnotation: "true",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc private endpoints by private annotation",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1": "value1",
							"annotation2": "value2",
						},
						Ports: []model.Port{
							{
								HostPort:      6379,
								ContainerPort: 6379,
								Protocol:      apiv1.ProtocolTCP,
							},
						},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Annotations: map[string]string{
						"annotation1": "value1",
						"annotation2": "value2",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackName",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-6379-6379-tcp",
							Port:       6379,
							TargetPort: intstr.IntOrString{IntVal: 6379},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateService("svcName", tt.stack)
			assert.Equal(t, tt.expected, result)
		})
	}

}

func Test_translateServiceIngress(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svc1": {
				Labels:      model.Labels{"label1": "value1"},
				Annotations: model.Annotations{"annotation1": "value1"},
				Image:       "image",
				Ports: []model.Port{
					{
						HostPort:      8080,
						ContainerPort: 8080,
					},
					{
						HostPort:      80,
						ContainerPort: 80,
					},
				},
			},
		},
	}
	result := translateServiceIngressV1("svc1-8080", "svc1", 8080, s)
	if result.Name != "svc1-8080" {
		t.Errorf("Wrong service name: '%s'", result.Name)
	}

	annotations := map[string]string{
		model.OktetoIngressAutoGenerateHost: "true",
		"annotation1":                       "value1",
	}

	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong service annotations: '%s'", result.Annotations)
	}

	pathType := networkingv1.PathTypeImplementationSpecific
	paths := []networkingv1.HTTPIngressPath{
		{
			Path:     "/",
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "svc1",
					Port: networkingv1.ServiceBackendPort{
						Number: 8080,
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(result.Spec.Rules[0].HTTP.Paths, paths) {
		t.Errorf("Wrong ingress: '%v'", result.Spec.Rules[0].HTTP.Paths)
	}

	labels := map[string]string{
		model.StackNameLabel: "stackName",
		"label1":             "value1",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong labels: '%s'", result.Labels)
	}
}

func Test_translateEndpointsV1(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Endpoints: map[string]model.Endpoint{
			"endpoint1": {
				Labels:      model.Labels{"label1": "value1"},
				Annotations: model.Annotations{"annotation1": "value1"},
				Rules: []model.EndpointRule{
					{Path: "/",
						Service: "svcName",
						Port:    80},
				},
			},
		},
		Services: map[string]*model.Service{
			"svcName": {
				Image: "image",
			},
		},
	}
	result := translateEndpointIngressV1("endpoint1", s)
	if result.Name != "endpoint1" {
		t.Errorf("Wrong service name: '%s'", result.Name)
	}

	annotations := map[string]string{
		model.OktetoIngressAutoGenerateHost: "true",
		"annotation1":                       "value1",
	}

	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong service annotations: '%s'", result.Annotations)
	}

	pathType := networkingv1.PathTypeImplementationSpecific
	paths := []networkingv1.HTTPIngressPath{
		{
			Path:     "/",
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "svcName",
					Port: networkingv1.ServiceBackendPort{
						Number: 80,
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(result.Spec.Rules[0].HTTP.Paths, paths) {
		t.Errorf("Wrong ingress: '%v'", result.Spec.Rules[0].HTTP.Paths)
	}

	labels := map[string]string{
		model.StackNameLabel:         "stackName",
		model.StackEndpointNameLabel: "endpoint1",
		"label1":                     "value1",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong labels: '%s'", result.Labels)
	}
}

func Test_translateEndpointsV1Beta1(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Endpoints: map[string]model.Endpoint{
			"endpoint1": {
				Labels:      model.Labels{"label1": "value1"},
				Annotations: model.Annotations{"annotation1": "value1"},
				Rules: []model.EndpointRule{
					{Path: "/",
						Service: "svcName",
						Port:    80},
				},
			},
		},
		Services: map[string]*model.Service{
			"svcName": {
				Image: "image",
			},
		},
	}
	result := translateEndpointIngressV1Beta1("endpoint1", s)
	if result.Name != "endpoint1" {
		t.Errorf("Wrong service name: '%s'", result.Name)
	}

	annotations := map[string]string{
		model.OktetoIngressAutoGenerateHost: "true",
		"annotation1":                       "value1",
	}

	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong service annotations: '%s'", result.Annotations)
	}

	paths := []networkingv1beta1.HTTPIngressPath{
		{Path: "/",
			Backend: networkingv1beta1.IngressBackend{
				ServiceName: "svcName",
				ServicePort: intstr.IntOrString{IntVal: 80},
			},
		},
	}

	if !reflect.DeepEqual(result.Spec.Rules[0].HTTP.Paths, paths) {
		t.Errorf("Wrong ingress: '%v'", result.Spec.Rules[0].HTTP.Paths)
	}

	labels := map[string]string{
		model.StackNameLabel:         "stackName",
		model.StackEndpointNameLabel: "endpoint1",
		"label1":                     "value1",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong labels: '%s'", result.Labels)
	}
}

func Test_translateSvcProbe(t *testing.T) {
	tests := []struct {
		name     string
		svc      *model.Service
		expected *apiv1.Probe
	}{
		{
			name: "nil healthcheck",
			svc: &model.Service{
				Healtcheck: nil,
			},
			expected: nil,
		},
		{
			name: "healthcheck http",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					HTTP: &model.HTTPHealtcheck{
						Path: "/",
						Port: 8080,
					},
				},
			},
			expected: &apiv1.Probe{
				ProbeHandler: apiv1.ProbeHandler{
					HTTPGet: &apiv1.HTTPGetAction{
						Path: "/",
						Port: intstr.IntOrString{IntVal: 8080},
					},
				},
			},
		},

		{
			name: "healthcheck http with other fields",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					HTTP: &model.HTTPHealtcheck{
						Path: "/",
						Port: 8080,
					},
					StartPeriod: 30 * time.Second,
					Retries:     5,
					Timeout:     5 * time.Minute,
					Interval:    45 * time.Second,
				},
			},
			expected: &apiv1.Probe{
				ProbeHandler: apiv1.ProbeHandler{
					HTTPGet: &apiv1.HTTPGetAction{
						Path: "/",
						Port: intstr.IntOrString{IntVal: 8080},
					},
				},
				InitialDelaySeconds: 30,
				FailureThreshold:    5,
				TimeoutSeconds:      300,
				PeriodSeconds:       45,
			},
		},
		{
			name: "healthcheck exec",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					Test: model.HealtcheckTest{
						"curl", "db-service:8080/readiness",
					},
				},
			},
			expected: &apiv1.Probe{
				ProbeHandler: apiv1.ProbeHandler{
					Exec: &apiv1.ExecAction{
						Command: []string{"curl", "db-service:8080/readiness"},
					},
				},
			},
		},
		{
			name: "healthcheck exec with others fields",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					Test: model.HealtcheckTest{
						"curl", "db-service:8080/readiness",
					},
					StartPeriod: 30 * time.Second,
					Retries:     5,
					Timeout:     5 * time.Minute,
					Interval:    45 * time.Second,
				},
			},
			expected: &apiv1.Probe{
				ProbeHandler: apiv1.ProbeHandler{
					Exec: &apiv1.ExecAction{
						Command: []string{"curl", "db-service:8080/readiness"},
					},
				},
				InitialDelaySeconds: 30,
				FailureThreshold:    5,
				TimeoutSeconds:      300,
				PeriodSeconds:       45,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := getSvcProbe(tt.svc)
			if !reflect.DeepEqual(tt.expected, probe) {
				t.Fatal("Wrong translation")
			}
		})
	}
}

func Test_translateServiceEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		svc      *model.Service
		expected []apiv1.EnvVar
	}{
		{
			name: "none",
			svc: &model.Service{
				Environment: model.Environment{},
			},
			expected: []apiv1.EnvVar{},
		},
		{
			name: "empty value",
			svc: &model.Service{
				Environment: model.Environment{
					model.EnvVar{
						Name: "DEBUG",
					},
				},
			},
			expected: []apiv1.EnvVar{
				{
					Name: "DEBUG",
				},
			},
		},
		{
			name: "empty name",
			svc: &model.Service{
				Environment: model.Environment{
					model.EnvVar{
						Value: "DEBUG",
					},
				},
			},
			expected: []apiv1.EnvVar{},
		},
		{
			name: "ok env var",
			svc: &model.Service{
				Environment: model.Environment{
					model.EnvVar{
						Name:  "DEBUG",
						Value: "true",
					},
				},
			},
			expected: []apiv1.EnvVar{
				{
					Name:  "DEBUG",
					Value: "true",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envs := translateServiceEnvironment(tt.svc)
			if !reflect.DeepEqual(tt.expected, envs) {
				t.Fatal("Wrong translation")
			}
		})
	}
}

func Test_translateAffinity(t *testing.T) {
	tests := []struct {
		name     string
		svc      *model.Service
		affinity *apiv1.Affinity
	}{
		{
			name: "none",
			svc: &model.Service{
				Environment: model.Environment{},
			},
			affinity: nil,
		},
		{
			name: "only volume mounts",
			svc: &model.Service{
				VolumeMounts: []model.StackVolume{
					{
						LocalPath:  "",
						RemotePath: "/var",
					},
				},
			},
			affinity: nil,
		},
		{
			name: "one volume",
			svc: &model.Service{
				Volumes: []model.StackVolume{
					{
						LocalPath:  "test",
						RemotePath: "/var",
					},
				},
			},
			affinity: &apiv1.Affinity{
				PodAffinity: &apiv1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple volumes",
			svc: &model.Service{
				Volumes: []model.StackVolume{
					{
						LocalPath:  "test-1",
						RemotePath: "/var",
					},
					{
						LocalPath:  "test-2",
						RemotePath: "/var",
					},
					{
						LocalPath:  "test-3",
						RemotePath: "/var",
					},
				},
			},
			affinity: &apiv1.Affinity{
				PodAffinity: &apiv1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test-1", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test-2", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test-3", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aff := translateAffinity(tt.svc)
			if !reflect.DeepEqual(tt.affinity, aff) {
				t.Fatal("Wrong translation")
			}
		})
	}
}

func TestGetSvcPublicPorts(t *testing.T) {
	tests := []struct {
		name           string
		svcName        string
		stack          *model.Stack
		expectedLength int
	}{
		{
			name:    "one public port",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Ports: []model.Port{
							{
								HostPort:      80,
								ContainerPort: 80,
							},
						},
					},
				},
			},
			expectedLength: 1,
		},
		{
			name:    "one private port",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Ports: []model.Port{
							{
								ContainerPort: 80,
							},
						},
					},
				},
			},
			expectedLength: 0,
		},
		{
			name:    "one public port with public field",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Public: true,
						Ports: []model.Port{
							{
								ContainerPort: 80,
								HostPort:      80,
							},
						},
					},
				},
			},
			expectedLength: 1,
		},
		{
			name:    "one public port",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Public: true,
						Ports: []model.Port{
							{
								HostPort:      80,
								ContainerPort: 80,
							},
						},
					},
				},
			},
			expectedLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := getSvcPublicPorts(tt.svcName, tt.stack)
			assert.Len(t, ports, tt.expectedLength)
		})
	}
}
