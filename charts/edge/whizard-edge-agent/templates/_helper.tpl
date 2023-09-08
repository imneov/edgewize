{{/*
Set Affinity according to edge cluster type
*/}}
{{- define "whizard-edge-agent.edgeNodeAffinity" -}}
{{- if eq .Values.clusterType "auto" }}
- key: node-role.kubernetes.io/edge
  operator: Exists
{{- end }}
{{- end }}