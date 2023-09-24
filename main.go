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
	httpHeaderKeyContentType               = "content-type"
	mimeTypeApplicationJson                = "application/json"
	annotationKeyPodAffinitySoft           = "kep-3633-alt.10h.in/podAffinity.preferredDuringSchedulingIgnoredDuringExecution"
	annotationKeyPodAffinityHard           = "kep-3633-alt.10h.in/podAffinity.requiredDuringSchedulingIgnoredDuringExecution"
	annotationKeyPodAntiAffinitySoft       = "kep-3633-alt.10h.in/podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution"
	annotationKeyPodAntiAffinityHard       = "kep-3633-alt.10h.in/podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution"
	annotationKeyTopologySpreadConstraints = "kep-3633-alt.10h.in/topologySpreadConstraints"
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
		CertFile:  "/certs/tls.crt",
		KeyFile:   "/certs/tls.key",
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
	var err error

	// HTTP request method validation
	if req.Method != http.MethodPost {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Application level request content validation and parsing

	// cluster admin level (i.e. webhook configuration level) validation and input extraction
	// When error found, response with error (4xx or 5xx)

	reqReview, clientErr, serverErr, errorMsg := validateExtractRequestReview(req.Body)
	if clientErr != nil {
		panic(handleClientError(resp, clientErr, errorMsg))
	}
	if serverErr != nil {
		panic(handleServerError(resp, serverErr, errorMsg))
	}

	reviewRequest := reqReview.Request
	reqObject, clientErr, serverErr, errorMsg := validateExtractRequestPod(reviewRequest)
	if clientErr != nil {
		panic(handleClientError(resp, clientErr, errorMsg))
	}
	if serverErr != nil {
		panic(handleServerError(resp, serverErr, errorMsg))
	}

	// cluster user level (i.e. request manifest level) validation and input extraction
	// When error found, response with OK (200), but notify error without HTTP response.
	// (for example: annotate pod with error message)

	labels := reqObject.GetLabels()
	annotations := reqObject.GetAnnotations()

	needPatch := false
	var exists bool

	hardPodAffinitySource, exists := annotations[annotationKeyPodAffinityHard]
	var hardAffinitiesAppending []corev1.PodAffinityTerm
	if exists {
		needPatch = true
		hardAffinitiesAppending, err = createHardAffinitiesAppending(hardPodAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create PodAffinityTerm: %#v", err)
			return
		}
	} else {
		hardAffinitiesAppending = make([]corev1.PodAffinityTerm, 0, 0)
	}

	var softAffinitiesAppending []corev1.WeightedPodAffinityTerm
	softPodAffinitySource, exists := annotations[annotationKeyPodAffinitySoft]
	if exists {
		needPatch = true
		softAffinitiesAppending, err = createSoftAffinitiesAppending(softPodAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create WeightedPodAffinityTerm: %#v", err)
			return
		}
	} else {
		softAffinitiesAppending = make([]corev1.WeightedPodAffinityTerm, 0, 0)
	}

	var hardAntiAffinitiesAppending []corev1.PodAffinityTerm
	hardPodAntiAffinitySource, exists := annotations[annotationKeyPodAntiAffinityHard]
	if exists {
		needPatch = true
		hardAntiAffinitiesAppending, err = createHardAffinitiesAppending(hardPodAntiAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create PodAffinityTerm: %#v", err)
			return
		}
	} else {
		hardAntiAffinitiesAppending = make([]corev1.PodAffinityTerm, 0, 0)
	}

	var softAntiAffinitiesAppending []corev1.WeightedPodAffinityTerm
	softPodAntiAffinitySource, exists := annotations[annotationKeyPodAntiAffinitySoft]
	if exists {
		needPatch = true
		softAntiAffinitiesAppending, err = createSoftAffinitiesAppending(softPodAntiAffinitySource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create WeightedPodAffinityTerm: %#v", err)
			return
		}
	} else {
		softAntiAffinitiesAppending = make([]corev1.WeightedPodAffinityTerm, 0, 0)
	}

	var topologySpreadConstraintsAppending []corev1.TopologySpreadConstraint
	topologySpreadConstraintsSource, exists := annotations[annotationKeyTopologySpreadConstraints]
	if exists {
		needPatch = true
		topologySpreadConstraintsAppending, err = createTopologySpreadConstraintsAppending(topologySpreadConstraintsSource, labels)
		if err != nil {
			// TODO: annotate pod with error message or publish events
			log.Printf("failed to create TopologySpreadConstraint: %#V", err)
			return
		}
	} else {
		topologySpreadConstraintsAppending = make([]corev1.TopologySpreadConstraint, 0, 0)
	}

	// create response content

	respReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: reqReview.APIVersion,
			Kind:       reqReview.Kind,
		},
		Response: &admissionv1.AdmissionResponse{
			Allowed: true,
			UID:     reviewRequest.UID,
		},
	}

	if needPatch {
		podAffinityPatch := createAffinityJSONPatch(reqObject, hardAffinitiesAppending, softAffinitiesAppending, hardAntiAffinitiesAppending, softAntiAffinitiesAppending)
		topologySpreadPatch := createTopologySpreadConstraintsJSONPatch(reqObject, topologySpreadConstraintsAppending)
		patch := append(podAffinityPatch, topologySpreadPatch...)

		var patchBytes []byte
		patchBytes, err = json.Marshal(patch)
		if err != nil {
			// TODO: return 500
		}

		respReview.Response.PatchType = &patchTypeJSONPatch
		respReview.Response.Patch = patchBytes
	}

	var respBytes []byte
	respBytes, err = json.Marshal(respReview)
	if err != nil {
		// TODO: return 500
	}

	// do response

	resp.WriteHeader(http.StatusOK)
	_, err = resp.Write(respBytes)
	if err != nil {
		log.Printf("failed to write response: %#v", err)
	}

}

func validateExtractRequestReview(reqBody io.Reader) (reqReview *admissionv1.AdmissionReview, clientErr, serverErr error, errorMessage string) {
	bodyBytes, err := io.ReadAll(reqBody)
	if err != nil {
		return nil, nil, err, "invalid request: failed to read body"
	}

	reqReview = &admissionv1.AdmissionReview{}
	err = json.Unmarshal(bodyBytes, reqReview)
	if err != nil {
		return nil, err, nil, "invalid request: failed to unmarshal request body"
	}

	reviewRequest := reqReview.Request
	if reviewRequest == nil {
		err = fmt.Errorf("request does not contain \"request\" field")
		return nil, err, nil, ""
	}

	if reviewRequest.Operation != admissionv1.Create {
		err = fmt.Errorf("handle CREATE operation only")
		return nil, err, nil, ""
	}

	if reviewRequest.Resource != podsv1GVR {
		err = fmt.Errorf("accept only core/v1/pods")
		return nil, err, nil, ""
	}

	if reviewRequest.SubResource != "" {
		err = fmt.Errorf("accept only core/v1/pods itself, not subresources")
		return nil, err, nil, ""
	}

	return reqReview, nil, nil, ""
}

func validateExtractRequestPod(reviewRequest *admissionv1.AdmissionRequest) (reqPod *corev1.Pod, clientErr, serverErr error, errorMessage string) {
	reqObject := &corev1.Pod{}
	clientErr = json.Unmarshal(reviewRequest.Object.Raw, reqObject)
	if clientErr != nil {
		return nil, clientErr, nil, "failed to unmarshal request.object as core/v1/pods"
	}

	return reqObject, nil, nil, ""
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
	var softAffinities []KEP3633WeightedPodAffinityTerm
	err := json.Unmarshal(([]byte)(source), &softAffinities)
	if err != nil {
		return nil, err
	}
	softAffinitiesAppending := make([]corev1.WeightedPodAffinityTerm, 0, len(softAffinities))
	for _, kep3633WeightedTerm := range softAffinities {
		weightedTerm := *(kep3633WeightedTerm.WeightedPodAffinityTerm.DeepCopy())
		weightedTerm.PodAffinityTerm = *(kep3633WeightedTerm.PodAffinityTerm.PodAffinityTerm.DeepCopy())
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

func createTopologySpreadConstraintsAppending(source string, labels map[string]string) ([]corev1.TopologySpreadConstraint, error) {
	var constraints []corev1.TopologySpreadConstraint
	err := json.Unmarshal(([]byte)(source), &constraints)
	if err != nil {
		return nil, err
	}

	constraintsAppending := make([]corev1.TopologySpreadConstraint, 0, len(constraints))
	for _, constraint := range constraints {
		constraintAppending := *constraint.DeepCopy()
		constraintAppending.MatchLabelKeys = nil
		labelSelector := constraintAppending.LabelSelector
		if labelSelector == nil {
			labelSelector = &metav1.LabelSelector{}
			constraintAppending.LabelSelector = labelSelector
		}
		matchExp := labelSelector.MatchExpressions
		if matchExp == nil {
			matchExp = make([]metav1.LabelSelectorRequirement, 0, len(constraint.MatchLabelKeys))
		}
		for _, matchLabelKey := range constraint.MatchLabelKeys {
			requirement := matchLabelKeyToRequirement(matchLabelKey, labels)
			if requirement != nil {
				matchExp = append(matchExp, *requirement)
			}
		}
		constraintsAppending = append(constraintsAppending, constraintAppending)
	}
	return constraintsAppending, nil
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

func createAffinityJSONPatch(reqObject *corev1.Pod, hardAffinitiesAppending []corev1.PodAffinityTerm, softAffinitiesAppending []corev1.WeightedPodAffinityTerm, hardAntiAffinitiesAppending []corev1.PodAffinityTerm, softAntiAffinitiesAppending []corev1.WeightedPodAffinityTerm) (patch []map[string]interface{}) {

	hardAffinitiesNeeded := hardAffinitiesAppending != nil && len(hardAffinitiesAppending) > 0
	softAffinitiesNeeded := softAffinitiesAppending != nil && len(softAffinitiesAppending) > 0
	hardAntiAffinitiesNeeded := hardAntiAffinitiesAppending != nil && len(hardAntiAffinitiesAppending) > 0
	softAntiAffinitiesNeeded := softAntiAffinitiesAppending != nil && len(softAntiAffinitiesAppending) > 0

	podAffinitiesNeeded := hardAffinitiesNeeded || softAffinitiesNeeded
	podAntiAffinitiesNeeded := hardAntiAffinitiesNeeded || softAntiAffinitiesNeeded

	affinitiesNeeded := podAffinitiesNeeded || podAntiAffinitiesNeeded

	patch = make([]map[string]interface{}, 0)

	var affinityField = reqObject.Spec.Affinity
	var podAffinityField *corev1.PodAffinity
	var podHardAffinityField []corev1.PodAffinityTerm
	var podSoftAffinityField []corev1.WeightedPodAffinityTerm
	var podAntiAffinityField *corev1.PodAntiAffinity
	var podHardAntiAffinityField []corev1.PodAffinityTerm
	var podSoftAntiAffinityField []corev1.WeightedPodAffinityTerm

	if affinitiesNeeded {
		if affinityField != nil {
			podAffinityField = affinityField.PodAffinity
			podAntiAffinityField = affinityField.PodAntiAffinity
		} else {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/affinity",
				"value": map[string]interface{}{},
			})
		}
	}

	if podAffinitiesNeeded {
		if podAffinityField != nil {
			podHardAffinityField = podAffinityField.RequiredDuringSchedulingIgnoredDuringExecution
			podSoftAffinityField = podAffinityField.PreferredDuringSchedulingIgnoredDuringExecution
		} else {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/affinity/podAffinity",
				"value": map[string]interface{}{},
			})
		}
	}

	if podAntiAffinitiesNeeded {
		if podAntiAffinityField != nil {
			podHardAntiAffinityField = podAntiAffinityField.RequiredDuringSchedulingIgnoredDuringExecution
			podSoftAntiAffinityField = podAntiAffinityField.PreferredDuringSchedulingIgnoredDuringExecution
		} else {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/affinity/podAntiAffinity",
				"value": map[string]interface{}{},
			})
		}
	}

	if hardAffinitiesNeeded {
		if hardAffinitiesAppending != nil {
			if podHardAffinityField == nil {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"path":  "/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution",
					"value": hardAffinitiesAppending,
				})
			} else {
				for _, a := range hardAffinitiesAppending {
					patch = append(patch, map[string]interface{}{
						"op":    "add",
						"path":  "/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution/-",
						"value": a,
					})
				}
			}
		}
	}

	if softAffinitiesNeeded {
		if podSoftAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution",
				"value": softAffinitiesAppending,
			})
		} else {
			for _, a := range softAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"path":  "/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution/-",
					"value": a,
				})
			}
		}
	}

	if hardAntiAffinitiesNeeded {
		if podHardAntiAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution",
				"value": hardAffinitiesAppending,
			})
		} else {
			for _, a := range hardAntiAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"path":  "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/-",
					"value": a,
				})
			}
		}
	}

	if softAntiAffinitiesNeeded {
		if podSoftAntiAffinityField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution",
				"value": softAntiAffinitiesAppending,
			})
		} else {
			for _, a := range softAntiAffinitiesAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"path":  "/spec/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution/-",
					"value": a,
				})
			}
		}
	}

	return patch
}

func createTopologySpreadConstraintsJSONPatch(reqObject *corev1.Pod, constraintsAppending []corev1.TopologySpreadConstraint) []map[string]interface{} {
	patch := make([]map[string]interface{}, 0, 0)

	var topologySpreadConstraintsField = reqObject.Spec.TopologySpreadConstraints
	if len(constraintsAppending) > 0 {
		if topologySpreadConstraintsField == nil {
			patch = append(patch, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/topologySpreadConstraints",
				"value": constraintsAppending,
			})
		} else {
			for _, a := range constraintsAppending {
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"path":  "/spec/topologySpreadConstraints/-",
					"value": a,
				})
			}
		}
	}

	return patch
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

func (w *serverWrapper) ListenAndServe() error {
	if w.EnableTLS {
		return w.Server.ListenAndServeTLS(w.CertFile, w.KeyFile)
	}
	return w.Server.ListenAndServe()
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
