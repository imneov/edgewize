{{/*
Return the proper image name
*/}}


{{- define "common.images.image" -}}
{{- $registryName := .global.imageRegistry -}}
{{- $repositoryName := .imageRoot.repository -}}
{{- $separator := ":" -}}
{{- $termination := .global.tag | toString -}}
{{- if and .imageRoot.registry (ne .imageRoot.registry "") }}
    {{- $registryName = .imageRoot.registry -}}
{{- end -}}
{{- if .imageRoot.tag }}
    {{- $termination = .imageRoot.tag | toString -}}
{{- end -}}
{{- if .imageRoot.digest }}
    {{- $separator = "@" -}}
    {{- $termination = .imageRoot.digest | toString -}}
{{- end -}}
{{- if $registryName }}
    {{- printf "%s/%s%s%s" $registryName $repositoryName $separator $termination -}}
{{- else -}}
    {{- printf "%s%s%s" $repositoryName $separator $termination -}}
{{- end -}}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "common.images.pullSecrets" -}}
  {{- $pullSecrets := list }}

  {{- if .global }}
    {{- range .global.imagePullSecrets -}}
      {{- $pullSecrets = append $pullSecrets . -}}
    {{- end -}}
  {{- end -}}

  {{- range .images -}}
    {{- range .pullSecrets -}}
      {{- $pullSecrets = append $pullSecrets . -}}
    {{- end -}}
  {{- end -}}

  {{- if (not (empty $pullSecrets)) }}
imagePullSecrets:
    {{- range $pullSecrets }}
  - name: {{ . }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- define "component.cloudImage" -}}
{{- if .global.defaultImageRegistry -}}
{{- trimSuffix "/" .global.defaultImageRegistry -}}/{{- .imageInfo.repository -}}:{{- .imageInfo.tag -}}
{{- else -}}
{{- .imageInfo.repository -}}:{{- .imageInfo.tag -}}
{{- end -}}
{{- end }}

{{- define "component.edgeImage" -}}
{{- $registry := .global.defaultImageRegistry -}}
{{- if .global.edgeImageRegistry }}
{{- $registry = .global.edgeImageRegistry -}}
{{- end -}}
{{- if $registry -}}
{{- trimSuffix "/" $registry -}}/{{- .imageInfo.repository -}}:{{- .imageInfo.tag -}}
{{- else -}}
{{- .imageInfo.repository -}}:{{- .imageInfo.tag -}}
{{- end -}}
{{- end }}