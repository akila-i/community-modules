{{/*
Copyright 2026 The OpenChoreo Authors
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Base name for namespaced resources (Deployment, ConfigMap, SA, PVC, Service).
*/}}
{{- define "events-collector.fullname" -}}
{{- default "events-collector" .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Name for cluster-scoped resources (ClusterRole/Binding). Suffixed with the
release namespace so two installs in different namespaces do not collide on a
single cluster-wide object.
*/}}
{{- define "events-collector.clusterScopedName" -}}
{{- printf "%s-%s" (include "events-collector.fullname" .) .Release.Namespace | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Service account name.
*/}}
{{- define "events-collector.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "events-collector.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "events-collector.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: openchoreo
{{ include "events-collector.selectorLabels" . }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "events-collector.selectorLabels" -}}
app.kubernetes.io/name: {{ include "events-collector.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Comma-separated list of extensions active in the service block: health_check,
the file_storage extension when persistence is on, plus every key defined in
extraExtensions (so a defined extra extension is always wired in — no separate
"names" list to keep in sync).
*/}}
{{- define "events-collector.serviceExtensions" -}}
{{- $exts := list "health_check" -}}
{{- if .Values.persistence.enabled -}}{{- $exts = append $exts "file_storage" -}}{{- end -}}
{{- range $name, $cfg := .Values.extraExtensions -}}{{- $exts = append $exts $name -}}{{- end -}}
{{- join ", " $exts -}}
{{- end -}}
