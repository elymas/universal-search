{{/*
Expand the name of the chart.
*/}}
{{- define "universal-search.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "universal-search.fullname" -}}
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
{{- define "universal-search.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "universal-search.labels" -}}
helm.sh/chart: {{ include "universal-search.chart" . }}
{{ include "universal-search.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels — used in matchLabels and pod templates.
*/}}
{{- define "universal-search.selectorLabels" -}}
app.kubernetes.io/name: {{ include "universal-search.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Fullname with component suffix — e.g. "usearch-api", "researcher"
Usage: {{ include "universal-search.componentFullname" (dict "Release" .Release "Chart" .Chart "Values" .Values "component" "api") }}
*/}}
{{- define "universal-search.componentFullname" -}}
{{- $base := include "universal-search.fullname" . }}
{{- printf "%s-%s" $base .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Component selector labels.
Usage: {{ include "universal-search.componentSelectorLabels" (dict "component" "api") }}
*/}}
{{- define "universal-search.componentSelectorLabels" -}}
app.kubernetes.io/name: {{ include "universal-search.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Component labels — common labels + component label.
*/}}
{{- define "universal-search.componentLabels" -}}
{{ include "universal-search.labels" . }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Image reference builder.
Requires Chart context for AppVersion fallback.
Usage: {{ include "universal-search.image" (dict "Values" .Values "Chart" .Chart "image" .Values.usearch.api.image) }}
*/}}
{{- define "universal-search.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .image.repository }}
{{- $chartVersion := "" }}
{{- if .Chart }}
{{- $chartVersion = .Chart.AppVersion | default "" }}
{{- end }}
{{- $tag := .image.tag | default .Values.imageTag | default $chartVersion | default "latest" }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Secret resolver — returns the appropriate env var source for a secret key
based on the active secret backend tier (D5).

Usage:
  {{ include "universal-search.secretEnvVar" (dict "root" . "key" "POSTGRES_PASSWORD" "component" "api") }}

Returns a dict with:
  - type: "value" | "secretKeyRef" | "externalSecret"
  - value: the value (tier 1) or secret ref info (tier 2/3)
*/}}
{{- define "universal-search.secretEnvVar" -}}
{{- $backend := .root.Values.secrets.backend }}
{{- if eq $backend "values" }}
{{/* Tier 1: value directly from values — dev/CI ONLY */}}
type: value
value: {{ index .root.Values.secretsValues .key | default "" | quote }}
{{- else if eq $backend "existingSecret" }}
{{/* Tier 2: reference pre-existing K8s Secret */}}
type: secretKeyRef
name: {{ printf "%s-%s" .root.Values.secrets.existingSecretPrefix .component | default (printf "%s-secrets" .root.Release.Name) }}
key: {{ .key }}
{{- else if eq $backend "externalSecrets" }}
{{/* Tier 3: ExternalSecret CRD manages the K8s Secret */}}
type: secretKeyRef
name: {{ printf "%s-%s" .root.Release.Name .component }}
key: {{ .key }}
{{- end }}
{{- end }}

{{/*
Generate env var entries for secrets based on active backend.
Returns a list of env entries suitable for Deployment env[] block.

Usage:
  {{ include "universal-search.secretEnvEntries" (dict "root" . "secretEnv" .Values.usearch.api.secretEnv "component" "api") }}
*/}}
{{- define "universal-search.secretEnvEntries" -}}
{{- $backend := .root.Values.secrets.backend }}
{{- range $key, $value := .secretEnv }}
{{- if ne $key "" }}
- name: {{ $key }}
{{- if eq $backend "values" }}
  value: {{ index $.root.Values.secretsValues (lower $key | replace "_" "") | default $value | quote }}
{{- else }}
  valueFrom:
    secretKeyRef:
      name: {{ printf "%s-%s" $.root.Values.secrets.existingSecretPrefix $.component | default (printf "%s-secrets" $.root.Release.Name) }}
      key: {{ $key }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{/*
ServiceAccount name for a component.
*/}}
{{- define "universal-search.serviceAccountName" -}}
{{- if .serviceAccount.create }}
{{- printf "%s-%s" (include "universal-search.fullname" .) .component | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .serviceAccount.name | default (printf "%s-%s" (include "universal-search.fullname" .) .component) }}
{{- end }}
{{- end }}

{{/*
Database URL helper — constructs DSN from values.
*/}}
{{- define "universal-search.databaseUrl" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "postgresql://%s:$(POSTGRES_PASSWORD)@%s-postgresql:%d/%s?sslmode=disable"
    .Values.postgresql.auth.username
    (include "universal-search.fullname" .)
    (int 5432)
    .Values.postgresql.auth.database
}}
{{- else }}
{{- printf "postgresql://%s:$(POSTGRES_PASSWORD)@%s:%d/%s?sslmode=disable"
    .Values.postgresql.auth.username
    .Values.postgresql.external.host
    (int .Values.postgresql.external.port)
    .Values.postgresql.external.database
}}
{{- end }}
{{- end }}

{{/*
Redis URL helper.
*/}}
{{- define "universal-search.redisUrl" -}}
{{- if .Values.redis.enabled }}
{{- printf "redis://%s-redis-master:%d/0" (include "universal-search.fullname" .) (int 6379) }}
{{- else }}
{{- printf "redis://%s:%d/0" .Values.redis.external.host (int .Values.redis.external.port) }}
{{- end }}
{{- end }}
