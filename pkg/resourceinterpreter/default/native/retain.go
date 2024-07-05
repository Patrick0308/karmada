/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package native

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// retentionInterpreter is the function that retains values from "observed" object.
type retentionInterpreter func(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error)

func getAllDefaultRetentionInterpreter() map[schema.GroupVersionKind]retentionInterpreter {
	s := make(map[schema.GroupVersionKind]retentionInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = retainWorkloadFields
	s[corev1.SchemeGroupVersion.WithKind(util.PodKind)] = retainPodFields
	s[corev1.SchemeGroupVersion.WithKind(util.ServiceKind)] = lifted.RetainServiceFields
	s[corev1.SchemeGroupVersion.WithKind(util.ServiceAccountKind)] = lifted.RetainServiceAccountFields
	s[corev1.SchemeGroupVersion.WithKind(util.PersistentVolumeClaimKind)] = retainPersistentVolumeClaimFields
	s[corev1.SchemeGroupVersion.WithKind(util.PersistentVolumeKind)] = retainPersistentVolumeFields
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = retainJobSelectorFields
	s[corev1.SchemeGroupVersion.WithKind(util.SecretKind)] = retainSecretServiceAccountToken
	return s
}

func retainPodFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	desiredPod := &corev1.Pod{}
	err := helper.ConvertToTypedObject(desired, desiredPod)
	if err != nil {
		return nil, fmt.Errorf("failed to convert desiredPod from unstructured object: %v", err)
	}

	clusterPod := &corev1.Pod{}
	err = helper.ConvertToTypedObject(observed, clusterPod)
	if err != nil {
		return nil, fmt.Errorf("failed to convert clusterPod from unstructured object: %v", err)
	}

	desiredPod.Spec.NodeName = clusterPod.Spec.NodeName
	desiredPod.Spec.ServiceAccountName = clusterPod.Spec.ServiceAccountName
	desiredPod.Spec.Volumes = clusterPod.Spec.Volumes
	// retain volumeMounts in each container
	for _, clusterContainer := range clusterPod.Spec.Containers {
		for desiredIndex, desiredContainer := range desiredPod.Spec.Containers {
			if desiredContainer.Name == clusterContainer.Name {
				desiredPod.Spec.Containers[desiredIndex].VolumeMounts = clusterContainer.VolumeMounts
				break
			}
		}
	}

	// retain volumeMounts in each init container
	for _, clusterInitContainer := range clusterPod.Spec.InitContainers {
		for desiredIndex, desiredInitContainer := range desiredPod.Spec.InitContainers {
			if desiredInitContainer.Name == clusterInitContainer.Name {
				desiredPod.Spec.InitContainers[desiredIndex].VolumeMounts = clusterInitContainer.VolumeMounts
				break
			}
		}
	}
	unstructuredObj, err := helper.ToUnstructured(desiredPod)
	if err != nil {
		return nil, fmt.Errorf("failed to transform Pod: %v", err)
	}

	return unstructuredObj, nil
}

func retainPersistentVolumeClaimFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// volumeName is allocated by member cluster and unchangeable, so it should be retained while updating
	volumeName, ok, err := unstructured.NestedString(observed.Object, "spec", "volumeName")
	if err != nil {
		return nil, fmt.Errorf("error retrieving volumeName from pvc: %w", err)
	}
	if ok && len(volumeName) > 0 {
		if err = unstructured.SetNestedField(desired.Object, volumeName, "spec", "volumeName"); err != nil {
			return nil, fmt.Errorf("error setting volumeName for pvc: %w", err)
		}
	}
	return desired, nil
}

func retainPersistentVolumeFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// claimRef is updated by kube-controller-manager.
	claimRef, ok, err := unstructured.NestedFieldNoCopy(observed.Object, "spec", "claimRef")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve claimRef from pv: %v", err)
	}
	if ok {
		if err = unstructured.SetNestedField(desired.Object, claimRef, "spec", "claimRef"); err != nil {
			return nil, fmt.Errorf("failed to set claimRef for pv: %v", err)
		}
	}
	return desired, nil
}

func retainJobSelectorFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	matchLabels, exist, err := unstructured.NestedStringMap(observed.Object, "spec", "selector", "matchLabels")
	if err != nil {
		return nil, err
	}
	if exist {
		err = unstructured.SetNestedStringMap(desired.Object, matchLabels, "spec", "selector", "matchLabels")
		if err != nil {
			return nil, err
		}
	}

	templateLabels, exist, err := unstructured.NestedStringMap(observed.Object, "spec", "template", "metadata", "labels")
	if err != nil {
		return nil, err
	}
	if exist {
		err = unstructured.SetNestedStringMap(desired.Object, templateLabels, "spec", "template", "metadata", "labels")
		if err != nil {
			return nil, err
		}
	}
	return desired, nil
}

func retainWorkloadFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	desiredDeploy := &appsv1.Deployment{}
	err := helper.ConvertToTypedObject(desired, desiredDeploy)
	if err != nil {
		return nil, fmt.Errorf("failed to convert desciredDeploy from unstructured object: %v", err)
	}

	observedDeploy := &appsv1.Deployment{}
	err = helper.ConvertToTypedObject(observed, observedDeploy)
	if err != nil {
		return nil, fmt.Errorf("failed to convert observedDeploy from unstructured object: %v", err)
	}

	err = setNewestRestartedAtTime(&desiredDeploy.Spec.Template.ObjectMeta, &observedDeploy.Spec.Template.ObjectMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to setNewestRestartedAtTime: %v", err)
	}

	if label, labelExist := desiredDeploy.Labels[util.RetainReplicasLabel]; labelExist && label == util.RetainReplicasValue {
		desiredDeploy.Spec.Replicas = observedDeploy.Spec.Replicas
	}

	unstructuredObj, err := helper.ToUnstructured(desiredDeploy)
	if err != nil {
		return nil, fmt.Errorf("failed to transform deployment: %v", err)
	}
	return unstructuredObj, nil
}

// setNewestRestartedAtTime sets the newest restartedAt time to desired object.
func setNewestRestartedAtTime(desiredMeta, observedMeta *metav1.ObjectMeta) error {
	desiredAnnos := desiredMeta.GetAnnotations()
	observedAnnos := observedMeta.GetAnnotations()
	// init annotations if desired object has no annotations
	observedRestarted, observedRestartedExist, err := getRestartedAtTime(observedAnnos)
	if err != nil {
		return err
	}
	// if observed object has no restartedAt annotation, no need to update desired objectMeta
	if !observedRestartedExist {
		return nil
	}
	// init annotations if desired object has no annotations
	if desiredAnnos == nil {
		desiredAnnos = make(map[string]string)
	}
	desiredRestarted, desiredRestartedExist, err := getRestartedAtTime(desiredAnnos)
	if err != nil {
		return err
	}
	// if desired object's restartedAt time is before observed object, update it
	if !desiredRestartedExist || desiredRestarted.Before(observedRestarted) {
		desiredAnnos["kubectl.kubernetes.io/restartedAt"] = observedAnnos["kubectl.kubernetes.io/restartedAt"]
		desiredMeta.SetAnnotations(desiredAnnos)
	}
	return nil
}

func getRestartedAtTime(annotations map[string]string) (time.Time, bool, error) {
	if annotations == nil {
		return time.Time{}, false, nil
	}
	restartAtTime, exist := annotations["kubectl.kubernetes.io/restartedAt"]
	if !exist {
		return time.Time{}, exist, nil
	}
	t, err := time.Parse(time.RFC3339, restartAtTime)
	if err != nil {
		return time.Time{}, exist, fmt.Errorf("failed to parse kubectl.kubernetes.io/restartedAt: %v", err)
	}
	return t, true, nil
}

func retainSecretServiceAccountToken(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	if secretType, exists, _ := unstructured.NestedString(desired.Object, "type"); exists && secretType == string(corev1.SecretTypeServiceAccountToken) {
		// retain token generated by cluster kube-controller-manager
		data, exist, err := unstructured.NestedStringMap(observed.Object, "data")
		if err != nil {
			return nil, fmt.Errorf("failed to get .data from desired.Object: %+v", err)
		}
		if exist {
			if err := unstructured.SetNestedStringMap(desired.Object, data, "data"); err != nil {
				return nil, fmt.Errorf("failed to set data for %s %s/%s", desired.GetKind(), desired.GetNamespace(), desired.GetName())
			}
		}
	}

	return desired, nil
}
