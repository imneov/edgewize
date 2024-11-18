{{/*
Expand the name of the chart.
*/}}
{{- define "model-service-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "model-service-controller.fullname" -}}
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
{{- define "model-service-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "model-service-controller.labels" -}}
helm.sh/chart: {{ include "model-service-controller.chart" . }}
{{ include "model-service-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "model-service-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "model-service-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "model-service-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "model-service-controller.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "model-mesh-cloud-component.image" -}}
{{- $registryName := .global.imageRegistry -}}
{{- $name := .component.name -}}
{{- $separator := ":" -}}
{{- $termination := "latest" | toString -}}
{{- if and .component.imageRegistry (ne .component.imageRegistry "") }}
    {{- $registryName = .component.imageRegistry -}}
{{- end -}}
{{- if .component.tag }}
    {{- $termination = .component.tag | toString -}}
{{- end -}}
{{- if .component.digest }}
    {{- $separator = "@" -}}
    {{- $termination = .component.digest | toString -}}
{{- end -}}
{{- $registryName = trimSuffix "/" $registryName  }}
{{- if and $registryName (ne $registryName "") }}
    {{- printf "%s/%s%s%s" $registryName $name $separator $termination -}}
{{- else -}}
    {{- printf "%s%s%s" $name $separator $termination -}}
{{- end -}}
{{- end -}}

{{- define "model-mesh-edge-component.image" -}}
{{- $registryName := .global.imageRegistry -}}
{{- if .global.edgeImageRegistry -}}
{{- $registryName = .global.edgeImageRegistry -}}
{{- end -}}
{{- $name := .component.name -}}
{{- $separator := ":" -}}
{{- $termination := "latest" | toString -}}
{{- if and .component.imageRegistry (ne .component.imageRegistry "") }}
    {{- $registryName = .component.imageRegistry -}}
{{- end -}}
{{- if .component.tag }}
    {{- $termination = .component.tag | toString -}}
{{- end -}}
{{- if .component.digest }}
    {{- $separator = "@" -}}
    {{- $termination = .component.digest | toString -}}
{{- end -}}
{{- $registryName = trimSuffix "/" $registryName  }}
{{- if and $registryName (ne $registryName "") }}
    {{- printf "%s/%s%s%s" $registryName $name $separator $termination -}}
{{- else -}}
    {{- printf "%s%s%s" $name $separator $termination -}}
{{- end -}}
{{- end -}}

{{- define "model-mesh-msc.image" -}}
{{ include "model-mesh-cloud-component.image" (dict "component" .Values.imageReference.modelMeshMsc "global" .Values) }}
{{- end -}}

{{- define "model-mesh-broker.image" -}}
{{ include "model-mesh-edge-component.image" (dict "component" .Values.imageReference.modelMeshBroker "global" .Values) }}
{{- end -}}

{{- define "model-mesh-proxy.image" -}}
{{ include "model-mesh-edge-component.image" (dict "component" .Values.imageReference.modelMeshProxy "global" .Values) }}
{{- end -}}

{{- define "model-mesh-component.imagePullSecrets" -}}
  {{- $pullSecrets := list }}

  {{- if .global.imagePullSecrets }}
    {{- range .global.imagePullSecrets -}}
      {{- $pullSecrets = append $pullSecrets . -}}
    {{- end -}}
  {{- end -}}

  {{- if .component.imagePullSecrets }}
    {{- range .component.imagePullSecrets -}}
      {{- $pullSecrets = append $pullSecrets . -}}
    {{- end -}}
  {{- end -}}

  {{- if (not (empty $pullSecrets)) }}
imagePullSecrets:
    {{- range $item := $pullSecrets }}
  - name: {{ $item.name }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- define "model-mesh-msc.imagePullSecrets" -}}
{{- include "model-mesh-component.imagePullSecrets" (dict "component" .Values.imageReference.modelMeshMsc "global" .Values) -}}
{{- end -}}

{{- define "model-mesh-broker.imagePullSecrets" -}}
{{- include "model-mesh-component.imagePullSecrets" (dict "component" .Values.imageReference.modelMeshBroker "global" .Values) -}}
{{- end -}}

{{- define "model-mesh-proxy.imagePullSecrets" -}}
{{- include "model-mesh-component.imagePullSecrets" (dict "component" .Values.imageReference.modelMeshProxy "global" .Values) -}}
{{- end -}}
