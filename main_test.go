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
	// testScaleIndex = 1 for normal tests. testScaleIndex = 2 make tests heavy.
	testScaleIndex = 1
)

type testCreateAffinityJSONPatchCase struct {
	BeforeHardAffinities        []corev1.PodAffinityTerm
	BeforeSoftAffinities        []corev1.WeightedPodAffinityTerm
	BeforeHardAntiAffinities    []corev1.PodAffinityTerm
	BeforeSoftAntiAffinities    []corev1.WeightedPodAffinityTerm
	HardAffinitiesAppending     []corev1.PodAffinityTerm
	SoftAffinitiesAppending     []corev1.WeightedPodAffinityTerm
	HardAntiAffinitiesAppending []corev1.PodAffinityTerm
	SoftAntiAffinitiesAppending []corev1.WeightedPodAffinityTerm
}

func TestCreateAffinityJSONPatch(t *testing.T) {
	testCases := make([]testCreateAffinityJSONPatchCase, 0, 1<<16)
	for bits := 0; bits < (1 << (8 * testScaleIndex)); bits++ {
		var size int
		testCase := testCreateAffinityJSONPatchCase{}

		mask := (1 << (testScaleIndex + 1)) - 1
		size = (bits >> (0 * testScaleIndex)) & mask
		testCase.BeforeHardAffinities = make([]corev1.PodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.BeforeHardAffinities[idx] = corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: fmt.Sprintf("topology.kubernetes.io/host%d", idx),
			}
		}
		size = (bits >> (1 * testScaleIndex)) & mask
		testCase.BeforeSoftAffinities = make([]corev1.WeightedPodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.BeforeSoftAffinities[idx] = corev1.WeightedPodAffinityTerm{
				Weight: 50,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "nginx",
						},
					},
					TopologyKey: fmt.Sprintf("topology.kubernetes.io/host%d", idx),
				},
			}
		}
		size = (bits >> (2 * testScaleIndex)) & mask
		testCase.BeforeHardAntiAffinities = make([]corev1.PodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.BeforeHardAntiAffinities[idx] = corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: fmt.Sprintf("topology.kubernetes.io/host%d", idx),
			}
		}
		size = (bits >> (3 * testScaleIndex)) & mask
		testCase.BeforeSoftAntiAffinities = make([]corev1.WeightedPodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.BeforeSoftAntiAffinities[idx] = corev1.WeightedPodAffinityTerm{
				Weight: 50,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "nginx",
						},
					},
					TopologyKey: fmt.Sprintf("topology.kubernetes.io/host%d", idx),
				},
			}
		}
		size = (bits >> (4 * testScaleIndex)) & mask
		testCase.HardAffinitiesAppending = make([]corev1.PodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.HardAffinitiesAppending[idx] = corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: fmt.Sprintf("topology.kubernetes.io/zone%d", idx),
			}
		}
		size = (bits >> (5 * testScaleIndex)) & mask
		testCase.SoftAffinitiesAppending = make([]corev1.WeightedPodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.SoftAffinitiesAppending[idx] = corev1.WeightedPodAffinityTerm{
				Weight: 50,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "nginx",
						},
					},
					TopologyKey: fmt.Sprintf("topology.kubernetes.io/zone%d", idx),
				},
			}
		}
		size = (bits >> (6 * testScaleIndex)) & mask
		testCase.HardAntiAffinitiesAppending = make([]corev1.PodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.HardAntiAffinitiesAppending[idx] = corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: fmt.Sprintf("topology.kubernetes.io/zone%d", idx),
			}
		}
		size = (bits >> (7 * testScaleIndex)) & mask
		testCase.SoftAntiAffinitiesAppending = make([]corev1.WeightedPodAffinityTerm, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.SoftAntiAffinitiesAppending[idx] = corev1.WeightedPodAffinityTerm{
				Weight: 50,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "nginx",
						},
					},
					TopologyKey: fmt.Sprintf("topology.kubernetes.io/zone%d", idx),
				},
			}
		}
		testCases = append(testCases, testCase)
	}

	for _, testCase := range testCases {
		pod := prepareBasicPod()
		if len(testCase.BeforeHardAffinities) > 0 {
			if pod.Spec.Affinity == nil {
				pod.Spec.Affinity = &corev1.Affinity{}
			}
			if pod.Spec.Affinity.PodAffinity == nil {
				pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
			}
			pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = testCase.BeforeHardAffinities
		}
		if len(testCase.BeforeSoftAffinities) > 0 {
			if pod.Spec.Affinity == nil {
				pod.Spec.Affinity = &corev1.Affinity{}
			}
			if pod.Spec.Affinity.PodAffinity == nil {
				pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
			}
			pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = testCase.BeforeSoftAffinities
		}
		if len(testCase.BeforeHardAntiAffinities) > 0 {
			if pod.Spec.Affinity == nil {
				pod.Spec.Affinity = &corev1.Affinity{}
			}
			if pod.Spec.Affinity.PodAntiAffinity == nil {
				pod.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
			}
			pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = testCase.BeforeHardAntiAffinities
		}
		if len(testCase.BeforeSoftAntiAffinities) > 0 {
			if pod.Spec.Affinity == nil {
				pod.Spec.Affinity = &corev1.Affinity{}
			}
			if pod.Spec.Affinity.PodAntiAffinity == nil {
				pod.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
			}
			pod.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = testCase.BeforeSoftAntiAffinities
		}

		patches := createAffinityJSONPatch(pod, testCase.HardAffinitiesAppending, testCase.SoftAffinitiesAppending, testCase.HardAntiAffinitiesAppending, testCase.SoftAntiAffinitiesAppending)

		patchedPod, err := applyPatch(pod, patches)
		if err != nil {
			t.Error("failed to patch pod with created JSONPatch", err)
		}

		if len(testCase.BeforeHardAffinities)+len(testCase.HardAffinitiesAppending) > 0 {
			if !hardAffinityFieldNonNil(patchedPod) {
				t.Error("/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution field not found")
			}
			expectedLen := len(testCase.BeforeHardAffinities) + len(testCase.HardAffinitiesAppending)
			actualLen := len(patchedPod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
			if actualLen != expectedLen {
				t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", expectedLen, actualLen)
			}
		}
		if len(testCase.BeforeSoftAffinities)+len(testCase.SoftAffinitiesAppending) > 0 {
			if !softAffinityFieldNonNil(patchedPod) {
				t.Error("/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution field not found")
			}
			expectedLen := len(testCase.BeforeSoftAffinities) + len(testCase.SoftAffinitiesAppending)
			actualLen := len(patchedPod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
			if actualLen != expectedLen {
				t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", expectedLen, actualLen)
			}
		}
		if len(testCase.BeforeHardAntiAffinities)+len(testCase.HardAntiAffinitiesAppending) > 0 {
			if !hardAntiAffinityFieldNonNil(patchedPod) {
				t.Error("/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution field not found")
			}
			expectedLen := len(testCase.BeforeHardAntiAffinities) + len(testCase.HardAntiAffinitiesAppending)
			actualLen := len(patchedPod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
			if actualLen != expectedLen {
				t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", expectedLen, actualLen)
			}
		}
		if len(testCase.BeforeSoftAntiAffinities)+len(testCase.SoftAntiAffinitiesAppending) > 0 {
			if !softAntiAffinityFieldNonNil(patchedPod) {
				t.Error("/spec/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution field not found")
			}
			expectedLen := len(testCase.BeforeSoftAntiAffinities) + len(testCase.SoftAntiAffinitiesAppending)
			actualLen := len(patchedPod.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
			if actualLen != expectedLen {
				t.Errorf("unexpected pod affinity size: expected: %d, actual: %d", expectedLen, actualLen)
			}
		}
	}
}

type testCreateTopologySpreadConstraintsJSONPatchCase struct {
	BeforeTopologySpreadConstraints    []corev1.TopologySpreadConstraint
	TopologySpreadconstraintsAppending []corev1.TopologySpreadConstraint
}

func TestCreateTopologySpreadConstrainsJSONPatch(t *testing.T) {
	testCaseSize := 1 << 4
	testCases := make([]testCreateTopologySpreadConstraintsJSONPatchCase, testCaseSize)
	for bits := 0; bits < testCaseSize; bits++ {
		testCase := testCreateTopologySpreadConstraintsJSONPatchCase{}
		var size int
		mask := (1 << (testScaleIndex + 1)) - 1
		size = (bits >> (0 * testScaleIndex)) & mask
		testCase.BeforeTopologySpreadConstraints = make([]corev1.TopologySpreadConstraint, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.BeforeTopologySpreadConstraints[idx] = corev1.TopologySpreadConstraint{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: fmt.Sprintf("topology.kubernetes.io/host%d", idx),
				MaxSkew:     1,
			}
		}
		size = (bits >> (1 * testScaleIndex)) & mask
		testCase.TopologySpreadconstraintsAppending = make([]corev1.TopologySpreadConstraint, size, size)
		for idx := 0; idx < size; idx++ {
			testCase.TopologySpreadconstraintsAppending[idx] = corev1.TopologySpreadConstraint{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				TopologyKey: fmt.Sprintf("topology.kubernetes.io/zone%d", idx),
				MaxSkew:     1,
			}
		}
		testCases = append(testCases, testCase)
	}

	for _, testCase := range testCases {
		pod := prepareBasicPod()
		if len(testCase.BeforeTopologySpreadConstraints) > 0 {
			pod.Spec.TopologySpreadConstraints = testCase.BeforeTopologySpreadConstraints
		}

		patches := createTopologySpreadConstraintsJSONPatch(pod, testCase.TopologySpreadconstraintsAppending)

		patchedPod, err := applyPatch(pod, patches)
		if err != nil {
			t.Error("failed to patch pod with created JSONPatch", err)
		}

		if len(testCase.BeforeTopologySpreadConstraints)+len(testCase.TopologySpreadconstraintsAppending) > 0 {
			if !topologySpreadConstraintsNonNil(patchedPod) {
				t.Error("/spec/topologySpreadConstraints field not found")
			}
			expectedLen := len(testCase.BeforeTopologySpreadConstraints) + len(testCase.TopologySpreadconstraintsAppending)
			actualLen := len(patchedPod.Spec.TopologySpreadConstraints)
			if actualLen != expectedLen {
				t.Errorf("unexpected pod topologySpreadConstraints size: expected: %d, actual: %d", expectedLen, actualLen)
			}
		}
	}
}

//
// utilities
//

func topologySpreadConstraintsNonNil(pod *corev1.Pod) bool {
	return pod.Spec.TopologySpreadConstraints != nil
}

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
