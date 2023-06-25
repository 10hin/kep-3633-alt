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
	HTTP_HEADER_KEY_CONTENT_TYPE = "content-type"
	MIME_TYPE_APPLICATION_JSON   = "application/json"
	//ANNOTATION_KEY_POD_AFFINITY_SOFT      = "kep-3633-alt.10h.in/podAffinity.preferredDuringSchedulingIgnoredDuringExecution"
	//ANNOTATION_KEY_POD_AFFINITY_HARD      = "kep-3633-alt.10h.in/podAffinity.requiredDuringSchedulingIgnoredDuringExecution"
	//ANNOTATION_KEY_POD_ANTI_AFFINITY_SOFT = "kep-3633-alt.10h.in/podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution"
	ANNOTATION_KEY_POD_ANTI_AFFINITY_HARD = "kep-3633-alt.10h.in/podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution"
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

	router := http.NewServeMux()
	router.HandleFunc("/", mutate)

	var addr string
	if *disableTLS {
		addr = ":8080"
	} else {
		addr = ":8443"
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
	log.Fatal(server.ListenAndServe())
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
	err = reqReview.Unmarshal(bodyBytes)
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

	if *reviewRequest.RequestResource != podsv1GVR {
		err = fmt.Errorf("accept only core/v1/pods")
		panic(handleClientError(resp, err, ""))
	}

	if reviewRequest.RequestSubResource != "" {
		err = fmt.Errorf("accept only core/v1/pods itself, not subresources")
		panic(handleClientError(resp, err, ""))
	}

	reqObject := &corev1.Pod{}
	err = reqObject.Unmarshal(reviewRequest.Object.Raw)
	if err != nil {
		panic(handleClientError(resp, err, "failed to unmarshal request.object as core/v1/pods"))
	}

	labels := reqObject.GetLabels()
	annotations := reqObject.GetAnnotations()

	var hardAntiAffinitiesAppending []corev1.PodAffinityTerm

	var hardPodAntiAffinitySource string
	var exists bool
	hardPodAntiAffinitySource, exists = annotations[ANNOTATION_KEY_POD_ANTI_AFFINITY_HARD]
	if exists {
		hardAntiAffinitiesAppending, err = createHardAffinitiesAppending(hardPodAntiAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create PodAffinityTerm: %#v", err)
			return
		}
	}

	patch := make([]map[string]interface{}, 0)

	var affinityField *corev1.Affinity
	//var podAffinityField *corev1.PodAffinity
	//var podHardAffinityField []corev1.PodAffinityTerm
	//var podSoftAffinityField []corev1.WeightedPodAffinityTerm
	var podAntiAffinityField *corev1.PodAntiAffinity
	var podHardAntiAffinityField []corev1.PodAffinityTerm
	//var podSoftAntiAffinityField []corev1.WeightedPodAffinityTerm
	if affinityField != nil {
		//podAffinityField = affinityField.PodAffinity
		podAntiAffinityField = affinityField.PodAntiAffinity
	}
	//if podAffinityField != nil {
	//	podHardAffinityField = podAffinityField.RequiredDuringSchedulingIgnoredDuringExecution
	//	podSoftAffinityField = podAffinityField.PreferredDuringSchedulingIgnoredDuringExecution
	//}
	if podAntiAffinityField != nil {
		podHardAntiAffinityField = podAntiAffinityField.RequiredDuringSchedulingIgnoredDuringExecution
		//podSoftAntiAffinityField = podAntiAffinityField.PreferredDuringSchedulingIgnoredDuringExecution
	}

	if hardAntiAffinitiesAppending != nil && len(hardAntiAffinitiesAppending) > 0 {
		if podHardAntiAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":     "add",
				"key":    "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution",
				"values": hardAntiAffinitiesAppending,
			})
		} else {
			for _, a := range hardAntiAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":     "add",
					"key":    "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/-",
					"values": a,
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
	log.Printf("failed to write response: %#v", err)

}

func createHardAffinitiesAppending(source string, labels map[string]string) ([]corev1.PodAffinityTerm, error) {
	var hardAntiAffinities []KEP3633PodAffinityTerm
	err := json.Unmarshal(([]byte)(source), &hardAntiAffinities)
	if err != nil {
		return nil, err
	}
	hardAntiAffinitiesAppending := make([]corev1.PodAffinityTerm, 0, len(hardAntiAffinities))
	for _, kep3633term := range hardAntiAffinities {
		term := *(kep3633term.PodAffinityTerm.DeepCopy())
		labelSelector := term.LabelSelector
		if labelSelector == nil {
			labelSelector = &metav1.LabelSelector{}
			term.LabelSelector = labelSelector
		}
		matchExp := labelSelector.MatchExpressions
		if matchExp != nil {
			matchExp = make([]metav1.LabelSelectorRequirement, 0, len(kep3633term.MatchLabelKeys))
			labelSelector.MatchExpressions = matchExp
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
		hardAntiAffinitiesAppending = append(hardAntiAffinitiesAppending, term)
	}
	return hardAntiAffinitiesAppending, nil
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
	resp.Header().Add(HTTP_HEADER_KEY_CONTENT_TYPE, MIME_TYPE_APPLICATION_JSON)
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
	resp.Header().Add(HTTP_HEADER_KEY_CONTENT_TYPE, MIME_TYPE_APPLICATION_JSON)
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
	corev1.WeightedPodAffinityTerm
	PodAffinityTerm KEP3633PodAffinityTerm `json:"podAffinityTerm:omitempty"`
}

type KEP3633PodAffinityTerm struct {
	corev1.PodAffinityTerm
	MatchLabelKeys    []string `json:"matchLabelKeys:omitempty"`
	MismatchLabelKeys []string `json:"mismatchLabelKeys:omitempty"`
}