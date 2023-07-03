{{/*
Expand the name of the chart.
*/}}
{{- define "kep3633alt.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kep3633alt.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "kep3633alt.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kep3633alt.labels" -}}
helm.sh/chart: {{ include "kep3633alt.chart" . }}
{{ include "kep3633alt.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kep3633alt.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kep3633alt.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Secret name for TLS certificates
*/}}
{{- define "kep3633alt.webhookCertSecret" -}}
{{- printf "%s-tls" (include "kep3633alt.fullname" .) -}}
{{- end -}}

{{/*
Generate certificates for webhook
*/}}
{{- define "kep3633alt.webhookCerts" -}}
{{- $serviceName := (include "kep3633alt.fullname" .) -}}
{{- $secretName := (include "kep3633alt.webhookCertSecret" .) -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace $secretName -}}
{{- if and .Values.keepTLSSecret $secret -}}
caCert: {{ index $secret.data "ca.crt" }}
clientCert: {{ index $secret.data "tls.crt" }}
clientKey: {{ index $secret.data "tls.key" }}
{{- else -}}
{{- $altNames := list (printf "%s.%s" $serviceName .Release.Namespace) (printf "%s.%s.svc" $serviceName .Release.Namespace) (printf "%s.%s.svc.%s" $serviceName .Release.Namespace .Values.cluster.dnsDomain) -}}
{{- $ca := genCA "kep3633alt-ca" 3650 -}}
{{- $cert := genSignedCert (include "kep3633alt.fullname" .) nil $altNames 3650 $ca -}}
caCert: {{ $ca.Cert | b64enc }}
clientCert: {{ $cert.Cert | b64enc }}
clientKey: {{ $cert.Key | b64enc }}
{{- end -}}
{{- end -}}
