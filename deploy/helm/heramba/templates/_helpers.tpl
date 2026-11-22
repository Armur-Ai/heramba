{{/*
Expand the name of the chart.
*/}}
{{- define "heramba.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a fully qualified app name.
*/}}
{{- define "heramba.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "heramba.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "heramba.labels" -}}
app.kubernetes.io/name: {{ include "heramba.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "heramba.selectorLabels" -}}
app.kubernetes.io/name: {{ include "heramba.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
