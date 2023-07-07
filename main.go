package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	httpHeaderKeyContentType         = "content-type"
	mimeTypeApplicationJson          = "application/json"
	annotationKeyPodAffinitySoft     = "kep-3633-alt.10h.in/podAffinity.preferredDuringSchedulingIgnoredDuringExecution"
	annotationKeyPodAffinityHard     = "kep-3633-alt.10h.in/podAffinity.requiredDuringSchedulingIgnoredDuringExecution"
	annotationKeyPodAntiAffinitySoft = "kep-3633-alt.10h.in/podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution"
	annotationKeyPodAntiAffinityHard = "kep-3633-alt.10h.in/podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution"
)

var (
	disableTLS = flag.Bool("disable-tls", false, "Disables")
	podsv1GVR  = metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	patchTypeJSONPatch = admissionv1.PatchTypeJSONPatch
)

func main() {
	flag.Parse()
	log.Println("start application...")

	router := http.NewServeMux()
	router.HandleFunc("/", mutate)
	router.HandleFunc("/healthz", health)

	var addr string
	if *disableTLS {
		addr = ":8080"
		log.Println("TLS disabled; listen address:", addr)
	} else {
		addr = ":8443"
		log.Println("TLS enabled; listen address:", addr)
	}
	server := serverWrapper{
		Server: http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		EnableTLS: !*disableTLS,
		CertFile:  "./tls.crt",
		KeyFile:   "./tls.key",
	}
	log.Println("start server", addr)
	log.Fatal(server.ListenAndServe())
}

func health(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write(([]byte)("{\"status\":\"UP\"}"))
}

func mutate(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		panic(handleClientError(resp, err, "invalid request: failed to read body"))
	}

	reqReview := &admissionv1.AdmissionReview{}
	err = json.Unmarshal(bodyBytes, reqReview)
	if err != nil {
		panic(handleClientError(resp, err, "invalid request: failed to unmarshal request body"))
	}

	reviewRequest := reqReview.Request
	if reviewRequest == nil {
		err = fmt.Errorf("request does not contain \"request\" field")
		panic(handleClientError(resp, err, ""))
	}

	if reviewRequest.Operation != admissionv1.Create {
		err = fmt.Errorf("handle CREATE operation only")
		panic(handleClientError(resp, err, ""))
	}

	if reviewRequest.Resource != podsv1GVR {
		err = fmt.Errorf("accept only core/v1/pods")
		panic(handleClientError(resp, err, ""))
	}

	if reviewRequest.SubResource != "" {
		err = fmt.Errorf("accept only core/v1/pods itself, not subresources")
		panic(handleClientError(resp, err, ""))
	}

	reqObject := &corev1.Pod{}
	err = json.Unmarshal(reviewRequest.Object.Raw, reqObject)
	if err != nil {
		panic(handleClientError(resp, err, "failed to unmarshal request.object as core/v1/pods"))
	}

	labels := reqObject.GetLabels()
	annotations := reqObject.GetAnnotations()

	var hardAffinitiesAppending []corev1.PodAffinityTerm
	var softAffinitiesAppending []corev1.WeightedPodAffinityTerm
	var hardAntiAffinitiesAppending []corev1.PodAffinityTerm
	var softAntiAffinitiesAppending []corev1.WeightedPodAffinityTerm

	var hardPodAffinitySource string
	var softPodAffinitySource string
	var hardPodAntiAffinitySource string
	var softPodAntiAffinitySource string
	var exists bool
	hardPodAffinitySource, exists = annotations[annotationKeyPodAffinityHard]
	if exists {
		hardAffinitiesAppending, err = createHardAffinitiesAppending(hardPodAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create PodAffinityTerm: %#v", err)
			return
		}
	}
	softPodAffinitySource, exists = annotations[annotationKeyPodAffinitySoft]
	if exists {
		softAffinitiesAppending, err = createSoftAffinitiesAppending(softPodAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create WeightedPodAffinityTerm: %#v", err)
			return
		}
	}
	hardPodAntiAffinitySource, exists = annotations[annotationKeyPodAntiAffinityHard]
	if exists {
		hardAntiAffinitiesAppending, err = createHardAffinitiesAppending(hardPodAntiAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create PodAffinityTerm: %#v", err)
			return
		}
	}
	softPodAntiAffinitySource, exists = annotations[annotationKeyPodAntiAffinitySoft]
	if exists {
		softAntiAffinitiesAppending, err = createSoftAffinitiesAppending(softPodAntiAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create WeightedPodAffinityTerm: %#v", err)
			return
		}
	}

	patch := make([]map[string]interface{}, 0)

	var affinityField = reqObject.Spec.Affinity
	var podAffinityField *corev1.PodAffinity
	var podHardAffinityField []corev1.PodAffinityTerm
	var podSoftAffinityField []corev1.WeightedPodAffinityTerm
	var podAntiAffinityField *corev1.PodAntiAffinity
	var podHardAntiAffinityField []corev1.PodAffinityTerm
	var podSoftAntiAffinityField []corev1.WeightedPodAffinityTerm
	if affinityField != nil {
		podAffinityField = affinityField.PodAffinity
		podAntiAffinityField = affinityField.PodAntiAffinity
	}
	if podAffinityField != nil {
		podHardAffinityField = podAffinityField.RequiredDuringSchedulingIgnoredDuringExecution
		podSoftAffinityField = podAffinityField.PreferredDuringSchedulingIgnoredDuringExecution
	}
	if podAntiAffinityField != nil {
		podHardAntiAffinityField = podAntiAffinityField.RequiredDuringSchedulingIgnoredDuringExecution
		podSoftAntiAffinityField = podAntiAffinityField.PreferredDuringSchedulingIgnoredDuringExecution
	}

	if hardAffinitiesAppending != nil && len(hardAffinitiesAppending) > 0 {
		if podHardAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"key":   "/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution",
				"value": hardAffinitiesAppending,
			})
		} else {
			for _, a := range hardAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"key":   "/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution/-",
					"value": a,
				})
			}
		}
	}

	if softAffinitiesAppending != nil && len(softAffinitiesAppending) > 0 {
		if podSoftAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"key":   "/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution",
				"value": softAffinitiesAppending,
			})
		} else {
			for _, a := range softAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"key":   "/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution/-",
					"value": a,
				})
			}
		}
	}

	if hardAntiAffinitiesAppending != nil && len(hardAntiAffinitiesAppending) > 0 {
		if podHardAntiAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"key":   "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution",
				"value": hardAffinitiesAppending,
			})
		} else {
			for _, a := range hardAntiAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"key":   "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/-",
					"value": a,
				})
			}
		}
	}

	if softAntiAffinitiesAppending != nil && len(softAntiAffinitiesAppending) > 0 {
		if podSoftAntiAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"key":   "/spec/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution",
				"value": softAntiAffinitiesAppending,
			})
		} else {
			for _, a := range softAntiAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"key":   "/spec/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution/-",
					"value": a,
				})
			}
		}
	}

	var patchBytes []byte
	patchBytes, err = json.Marshal(patch)
	if err != nil {
		// TODO: return 500
	}
	respReview := admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			Allowed:   true,
			UID:       reviewRequest.UID,
			PatchType: &patchTypeJSONPatch,
			Patch:     patchBytes,
		},
	}

	var respBytes []byte
	respBytes, err = json.Marshal(respReview)
	if err != nil {
		// TODO: return 500
	}

	resp.WriteHeader(http.StatusOK)
	_, err = resp.Write(respBytes)
	if err != nil {
		log.Printf("failed to write response: %#v", err)
	}

}

func createHardAffinitiesAppending(source string, labels map[string]string) ([]corev1.PodAffinityTerm, error) {
	var hardAffinities []KEP3633PodAffinityTerm
	err := json.Unmarshal(([]byte)(source), &hardAffinities)
	if err != nil {
		return nil, err
	}
	hardAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, len(hardAffinities))
	for _, kep3633term := range hardAffinities {
		term := *(kep3633term.PodAffinityTerm.DeepCopy())
		labelSelector := term.LabelSelector
		if labelSelector == nil {
			labelSelector = &metav1.LabelSelector{}
		}
		matchExp := labelSelector.MatchExpressions
		if matchExp == nil {
			matchExp = make([]metav1.LabelSelectorRequirement, 0, len(kep3633term.MatchLabelKeys))
		}
		for _, k := range kep3633term.MatchLabelKeys {
			requirement := matchLabelKeyToRequirement(k, labels)
			if requirement != nil {
				matchExp = append(matchExp, *requirement)
			}
		}
		for _, k := range kep3633term.MismatchLabelKeys {
			requirement := mismatchLabelKeyToRequirement(k, labels)
			if requirement != nil {
				matchExp = append(matchExp, *requirement)
			}
		}
		labelSelector.MatchExpressions = matchExp
		term.LabelSelector = labelSelector
		hardAffinitiesAppending = append(hardAffinitiesAppending, term)
	}
	return hardAffinitiesAppending, nil
}

func createSoftAffinitiesAppending(source string, labels map[string]string) ([]corev1.WeightedPodAffinityTerm, error) {
	log.Printf("[DEBUG] source: %#v", source)
	var softAffinities []KEP3633WeightedPodAffinityTerm
	err := json.Unmarshal(([]byte)(source), &softAffinities)
	if err != nil {
		return nil, err
	}
	log.Printf("[DEBUG] softAffinities: %#v", softAffinities)
	softAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, len(softAffinities))
	for _, kep3633WeightedTerm := range softAffinities {
		weightedTerm := *(kep3633WeightedTerm.WeightedPodAffinityTerm.DeepCopy())
		weightedTerm.PodAffinityTerm = *(kep3633WeightedTerm.PodAffinityTerm.PodAffinityTerm.DeepCopy())
		log.Printf("[DEBUG] weightedTerm: %#v", weightedTerm)
		labelSelector := weightedTerm.PodAffinityTerm.LabelSelector
		if labelSelector == nil {
			labelSelector = &metav1.LabelSelector{}
		}
		matchExp := labelSelector.MatchExpressions
		if matchExp == nil {
			matchExp = make([]metav1.LabelSelectorRequirement, 0, len(kep3633WeightedTerm.PodAffinityTerm.MatchLabelKeys))
		}
		for _, k := range kep3633WeightedTerm.PodAffinityTerm.MatchLabelKeys {
			requirement := matchLabelKeyToRequirement(k, labels)
			if requirement != nil {
				matchExp = append(matchExp, *requirement)
			}
		}
		for _, k := range kep3633WeightedTerm.PodAffinityTerm.MismatchLabelKeys {
			requirement := mismatchLabelKeyToRequirement(k, labels)
			if requirement != nil {
				matchExp = append(matchExp, *requirement)
			}
		}
		labelSelector.MatchExpressions = matchExp
		weightedTerm.PodAffinityTerm.LabelSelector = labelSelector
		softAffinitiesAppending = append(softAffinitiesAppending, weightedTerm)
	}
	return softAffinitiesAppending, nil
}

func matchLabelKeyToRequirement(matchLabelKey string, labels map[string]string) *metav1.LabelSelectorRequirement {
	v, exists := labels[matchLabelKey]
	if exists {
		return &metav1.LabelSelectorRequirement{
			Key:      matchLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{v},
		}
	} else {
		// TODO: If matchLabelKeys has key "foo", but the pod labels has no "foo" label, should affinity contain any terms?
		// TODO: For example:
		// matchExp = append(matchExp, metav1.LabelSelectorRequirement{
		// 	Key:      k,
		// 	Operator: metav1.LabelSelectorOpDoesNotExist,
		// })
		return nil
	}
}

func mismatchLabelKeyToRequirement(matchLabelKey string, labels map[string]string) *metav1.LabelSelectorRequirement {
	v, exists := labels[matchLabelKey]
	if exists {
		return &metav1.LabelSelectorRequirement{
			Key:      matchLabelKey,
			Operator: metav1.LabelSelectorOpNotIn,
			Values:   []string{v},
		}
	} else {
		// TODO: If matchLabelKeys has key "foo", but the pod labels has no "foo" label, should affinity contain any terms?
		// TODO: For example:
		// matchExp = append(matchExp, metav1.LabelSelectorRequirement{
		// 	Key:      k,
		// 	Operator: metav1.LabelSelectorOpDoesNotExist,
		// })
		return nil
	}
}

func handleClientError(resp http.ResponseWriter, respError error, msg string) error {
	bodyBytes, err := json.Marshal(errorBody{
		Error:   respError,
		Message: msg,
	})
	if err != nil {
		log.Printf("failed to format error to JSON response: %#v; original error: %#v", err, respError)
		return handleServerError(resp, err, "server failure")
	}
	resp.Header().Add(httpHeaderKeyContentType, mimeTypeApplicationJson)
	resp.WriteHeader(400)
	_, err = resp.Write(bodyBytes)
	if err != nil {
		log.Printf("failed to send error to client: %#v; original error: %#v", err, respError)
		return err
	}
	if len(msg) == 0 {
		return respError
	}
	return fmt.Errorf("%s: %w", msg, respError)
}

func handleServerError(resp http.ResponseWriter, respError error, msg string) error {
	bodyBytes, err := json.Marshal(errorBody{
		Error:   respError,
		Message: msg,
	})
	if err != nil {
		resp.WriteHeader(500)
		_, _ = resp.Write(([]byte)("server failure"))
		return err
	}
	resp.Header().Add(httpHeaderKeyContentType, mimeTypeApplicationJson)
	resp.WriteHeader(400)
	_, err = resp.Write(bodyBytes)
	if err != nil {
		log.Printf("failed to send error to client: %#v; original error: %#v", err, respError)
		return err
	}
	if len(msg) == 0 {
		return respError
	}
	return fmt.Errorf("%s: %w", msg, respError)
}

type errorBody struct {
	Error   error  `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

type serverWrapper struct {
	http.Server
	EnableTLS bool
	CertFile  string
	KeyFile   string
}

func (w *serverWrapper) Serve(l net.Listener) error {
	if w.EnableTLS {
		return w.Server.ServeTLS(l, w.CertFile, w.KeyFile)
	}
	return w.Server.Serve(l)
}

type KEP3633WeightedPodAffinityTerm struct {
	corev1.WeightedPodAffinityTerm `json:",inline"`
	PodAffinityTerm                KEP3633PodAffinityTerm `json:"podAffinityTerm,omitempty"`
}

type KEP3633PodAffinityTerm struct {
	corev1.PodAffinityTerm `json:",inline"`
	MatchLabelKeys         []string `json:"matchLabelKeys,omitempty"`
	MismatchLabelKeys      []string `json:"mismatchLabelKeys,omitempty"`
}
