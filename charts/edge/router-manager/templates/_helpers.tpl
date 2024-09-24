{{/*
Expand the name of the chart.
*/}}
{{- define "router-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "router-manager-edge.name" -}}
{{- printf "%s-edge" (default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-") }}
{{- end }}

{{- define "router-apiserver.name" -}}
{{- default .Values.apiserver.name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "router-controller.name" -}}
{{- default .Values.controller.name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "router-manager.fullname" -}}
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


{{- define "router-apiserver.fullname" -}}
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

{{- define "router-manager-edge.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- printf "%s-edge" (.Values.fullnameOverride | trunc 63 | trimSuffix "-") }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- printf "%s-edge" (.Release.Name | trunc 63 | trimSuffix "-") }}
{{- else }}
{{- printf "%s-%s-edge" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "router-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "router-controller.chart" -}}
{{- printf "%s-%s" .Values.controller.name .Values.controller.version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "router-apiserver.chart" -}}
{{- printf "%s-%s" .Values.apiserver.name .Values.apiserver.version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "router-manager.labels" -}}
helm.sh/chart: {{ include "router-manager.chart" . }}
{{ include "router-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "router-manager-edge.labels" -}}
helm.sh/chart: {{ include "router-manager.chart" . }}
{{ include "router-manager-edge.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "router-apiserver.labels" -}}
helm.sh/chart: {{ include "router-apiserver.chart" . }}
{{ include "router-apiserver.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "router-controller.labels" -}}
helm.sh/chart: {{ include "router-controller.chart" . }}
{{ include "router-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "router-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "router-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "router-manager-edge.selectorLabels" -}}
app.kubernetes.io/name: {{ include "router-manager-edge.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "router-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "router-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: {{ include "router-controller.name" . }}
{{- end }}

{{- define "router-apiserver.selectorLabels" -}}
app.kubernetes.io/name: {{ include "router-apiserver.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: {{ include "router-apiserver.name" . }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "router-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "router-manager.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
