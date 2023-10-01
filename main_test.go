package main

import (
	"encoding/json"
	"fmt"
	jsonpatch "gopkg.in/evanphx/json-patch.v5"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	jsonpatchKeyOperation      = "op"
	jsonpatchKeyPath           = "path"
	jsonpatchKeyValue          = "value"
	jsonpatchOperationValueAdd = "add"
)

func TestCreateAffinityJSONPatch_addNewHardAffinity(t *testing.T) {
	pod := prepareBasicPod()
	hardAffinitiesAppending := []corev1.PodAffinityTerm{
		{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			TopologyKey: "topology.kubernetes.io/zone",
		},
	}
	softAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, 0)
	hardAntiAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, 0)
	softAntiAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, 0)
	patches := createAffinityJSONPatch(pod, hardAffinitiesAppending, softAffinitiesAppending, hardAntiAffinitiesAppending, softAntiAffinitiesAppending)
	fmt.Println(patches)

	patchedPod, err := applyPatch(pod, patches)
	if err != nil {
		t.Error("failed to patch pod with created JSONPatch", err)
	}

	// assertions
	if !hardAffinityFieldNonNil(patchedPod) {
		t.Error("/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution field not found")
	}
	hardAffinityLen := len(patchedPod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
	if hardAffinityLen != len(hardAffinitiesAppending) {
		t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", len(hardAffinitiesAppending), hardAffinityLen)
	}
}

func TestCreateAffinityJSONPatch_addNewSoftAffinity(t *testing.T) {
	pod := prepareBasicPod()
	hardAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, 0)
	softAffinitiesAppending := []corev1.WeightedPodAffinityTerm{
		{
			Weight: 50,
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: "topology.kubernetes.io/zone",
			},
		},
	}
	hardAntiAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, 0)
	softAntiAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, 0)
	patches := createAffinityJSONPatch(pod, hardAffinitiesAppending, softAffinitiesAppending, hardAntiAffinitiesAppending, softAntiAffinitiesAppending)

	patchedPod, err := applyPatch(pod, patches)
	if err != nil {
		t.Error("failed to patch pod with created JSONPatch", err)
	}

	// assertions
	if !softAffinityFieldNonNil(patchedPod) {
		t.Error("/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution field not found")
	}
	softAffinityLen := len(patchedPod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
	if softAffinityLen != len(softAffinitiesAppending) {
		t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", len(softAffinitiesAppending), softAffinityLen)
	}
}

func TestCreateAffinityJSONPatch_addNewHardAntiAffinity(t *testing.T) {
	pod := prepareBasicPod()
	hardAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, 0)
	softAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, 0)
	hardAntiAffinitiesAppending := []corev1.PodAffinityTerm{
		{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			TopologyKey: "topology.kubernetes.io/zone",
		},
	}
	softAntiAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, 0)
	patches := createAffinityJSONPatch(pod, hardAffinitiesAppending, softAffinitiesAppending, hardAntiAffinitiesAppending, softAntiAffinitiesAppending)
	fmt.Println(patches)

	patchedPod, err := applyPatch(pod, patches)
	if err != nil {
		t.Error("failed to patch pod with created JSONPatch", err)
	}

	// assertions
	if !hardAntiAffinityFieldNonNil(patchedPod) {
		t.Error("/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution field not found")
	}
	hardAntiAffinityLen := len(patchedPod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
	if hardAntiAffinityLen != len(hardAntiAffinitiesAppending) {
		t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", len(hardAntiAffinitiesAppending), hardAntiAffinityLen)
	}
}

func TestCreateAffinityJSONPatch_addNewSoftAntiAffinity(t *testing.T) {
	pod := prepareBasicPod()
	hardAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, 0)
	softAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, 0)
	hardAntiAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, 0)
	softAntiAffinitiesAppending := []corev1.WeightedPodAffinityTerm{
		{
			Weight: 50,
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: "topology.kubernetes.io/zone",
			},
		},
	}
	patches := createAffinityJSONPatch(pod, hardAffinitiesAppending, softAffinitiesAppending, hardAntiAffinitiesAppending, softAntiAffinitiesAppending)

	patchedPod, err := applyPatch(pod, patches)
	if err != nil {
		t.Error("failed to patch pod with created JSONPatch", err)
	}

	// assertions
	if !softAntiAffinityFieldNonNil(patchedPod) {
		t.Error("/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution field not found")
	}
	softAntiAffinityLen := len(patchedPod.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
	if softAntiAffinityLen != len(softAntiAffinitiesAppending) {
		t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", len(softAntiAffinitiesAppending), softAntiAffinityLen)
	}
}

// utilities

func affinityFieldNonNil(pod *corev1.Pod) bool {
	return pod.Spec.Affinity != nil
}

func podAffinityFieldNonNil(pod *corev1.Pod) bool {
	if !affinityFieldNonNil(pod) {
		return false
	}
	return pod.Spec.Affinity.PodAffinity != nil
}

func hardAffinityFieldNonNil(pod *corev1.Pod) bool {
	if !podAffinityFieldNonNil(pod) {
		return false
	}
	return pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil
}

func softAffinityFieldNonNil(pod *corev1.Pod) bool {
	if !podAffinityFieldNonNil(pod) {
		return false
	}
	return pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil
}

func podAntiAffinityFieldNonNil(pod *corev1.Pod) bool {
	if !affinityFieldNonNil(pod) {
		return false
	}
	return pod.Spec.Affinity.PodAntiAffinity != nil
}

func hardAntiAffinityFieldNonNil(pod *corev1.Pod) bool {
	if !podAntiAffinityFieldNonNil(pod) {
		return false
	}
	return pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil
}

func softAntiAffinityFieldNonNil(pod *corev1.Pod) bool {
	if !podAntiAffinityFieldNonNil(pod) {
		return false
	}
	return pod.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil
}

func applyPatch(pod *corev1.Pod, patch []map[string]interface{}) (*corev1.Pod, error) {

	patchDoc, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("error while encoding patch object into JSONPatch document: %w", err)
	}

	patchObj, err := jsonpatch.DecodePatch(patchDoc)
	if err != nil {
		return nil, fmt.Errorf("error while decoding JSONPatch into patch object: %w", err)
	}

	podJSON, err := json.Marshal(pod)
	if err != nil {
		return nil, fmt.Errorf("error while encoding pod into JSON: %w", err)
	}

	patchedPodJSON, err := patchObj.Apply(podJSON)
	if err != nil {
		return nil, fmt.Errorf("error while applying patch: %w", err)
	}

	var patchedPod corev1.Pod
	err = json.Unmarshal(patchedPodJSON, &patchedPod)
	if err != nil {
		return nil, fmt.Errorf("error while decoding patched Pod from JSON: %w", err)
	}

	return &patchedPod, nil

}

func prepareBasicPod() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-00000000-0000",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:mainline-alpine",
					Ports: []corev1.ContainerPort{
						{
							Name:          "web",
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
		},
	}
}
