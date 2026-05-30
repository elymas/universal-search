{{/*
SPEC-DEPLOY-001 — shared helpers. Single source of truth for naming, labels,
image references, and secret resolution (REQ-DEPLOY-004: helpers live ONLY here).
*/}}

{{/* Base chart name, truncated to 63 chars (k8s name limit). */}}
{{- define "usearch.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Fully-qualified release name. */}}
{{- define "usearch.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* Chart label "name-version". */}}
{{- define "usearch.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels applied to every resource. Usage: {{ include "usearch.labels" . }}
*/}}
{{- define "usearch.labels" -}}
helm.sh/chart: {{ include "usearch.chart" . }}
{{ include "usearch.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/* Release-level selector labels (NOT per-component). */}}
{{- define "usearch.selectorLabels" -}}
app.kubernetes.io/name: {{ include "usearch.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Per-component name: "<fullname>-<component>". Call with a dict:
  {{ include "usearch.componentName" (dict "root" . "component" "api") }}
*/}}
{{- define "usearch.componentName" -}}
{{- printf "%s-%s" (include "usearch.fullname" .root) .component | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Per-component labels (release labels + component label). Call:
  {{ include "usearch.componentLabels" (dict "root" . "component" "api") }}
*/}}
{{- define "usearch.componentLabels" -}}
{{ include "usearch.labels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{/*
Per-component selector labels. Call:
  {{ include "usearch.componentSelectorLabels" (dict "root" . "component" "api") }}
*/}}
{{- define "usearch.componentSelectorLabels" -}}
{{ include "usearch.selectorLabels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{/*
Image reference. Call with a dict carrying root + per-service image map:
  {{ include "usearch.image" (dict "root" . "image" .Values.usearch.api.image) }}
Honors global.imageRegistry override (REQ-DEPLOY, D7 registry placeholder).
*/}}
{{- define "usearch.image" -}}
{{- $registry := .image.registry -}}
{{- if .root.Values.global.imageRegistry -}}
{{- $registry = .root.Values.global.imageRegistry -}}
{{- end -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry .image.repository (.image.tag | toString) -}}
{{- else -}}
{{- printf "%s:%s" .image.repository (.image.tag | toString) -}}
{{- end -}}
{{- end -}}

{{/* imagePullSecrets block (global). */}}
{{- define "usearch.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ .name | default . }}
{{- end }}
{{- end }}
{{- end -}}

{{/*
Name of the K8s Secret holding sensitive env. Tier 1 (values) → chart-managed
Secret "<fullname>-secrets"; tier 2 (existingSecret) → operator-provided name.
SPEC-DEPLOY-001 D5 / REQ-DEPLOY-016.
*/}}
{{- define "usearch.secretName" -}}
{{- if eq .Values.secrets.backend "existingSecret" -}}
{{- required "secrets.existingSecret.name is required when secrets.backend=existingSecret" .Values.secrets.existingSecret.name -}}
{{- else -}}
{{- printf "%s-secrets" (include "usearch.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Tier-3 guard. external-secrets backend is RESERVED but install-blocked in V1
(REQ-DEPLOY-023, depends on SEC-001 PR#42). Fails the render with a clear message.
Invoke once near the top of a rendered template (e.g. api/deployment.yaml).
*/}}
{{- define "usearch.assertSecretBackend" -}}
{{- if eq .Values.secrets.backend "externalSecrets" -}}
{{- fail "secrets.backend=externalSecrets (tier-3 ExternalSecrets) is a V1.1 feature and is install-blocked in V1 (depends on SPEC-SEC-001). Use secrets.backend=existingSecret for production or values for dev. See NOTES.txt." -}}
{{- end -}}
{{- end -}}

{{/*
ServiceAccount name for a component.
  {{ include "usearch.serviceAccountName" (dict "root" . "component" "api" "svc" .Values.usearch.api) }}
*/}}
{{- define "usearch.serviceAccountName" -}}
{{- if .svc.serviceAccount.create -}}
{{- default (include "usearch.componentName" (dict "root" .root "component" .component)) .svc.serviceAccount.name -}}
{{- else -}}
{{- default "default" .svc.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Prometheus scrape pod annotations — fallback emitted when the Prometheus Operator
ServiceMonitor CRD is absent (REQ-DEPLOY-019, D6). Call with the metrics port:
  {{ include "usearch.scrapeAnnotations" (dict "root" . "port" 8080 "path" "/metrics") }}
*/}}
{{- define "usearch.scrapeAnnotations" -}}
{{- if and .root.Values.observability.serviceMonitor.enabled (eq .root.Values.observability.serviceMonitor.fallback "annotations") -}}
prometheus.io/scrape: "true"
prometheus.io/port: {{ .port | quote }}
prometheus.io/path: {{ .path | default "/metrics" | quote }}
{{- end -}}
{{- end -}}
