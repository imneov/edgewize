{{- define "hami-scheduler-component.image" -}}
{{- $defaultImageRegistry := index . 0 -}}
{{- $image := index . 1 -}}
{{- $tag := index . 2 -}}
{{- if $defaultImageRegistry -}}
{{- trimSuffix "/" $defaultImageRegistry -}}/{{- $image -}}:{{- $tag -}}
{{- else -}}
{{- $image -}}:{{- $tag -}}
{{- end -}}
{{- end -}}