{{- define "runtime-incident-platform.name" -}}{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}{{- end }}
{{- define "runtime-incident-platform.fullname" -}}{{ default (include "runtime-incident-platform.name" .) .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}{{- end }}
{{- define "runtime-incident-platform.labels" -}}
app.kubernetes.io/name: {{ include "runtime-incident-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}
{{- define "runtime-incident-platform.serviceAccountName" -}}{{ default (include "runtime-incident-platform.fullname" .) .Values.serviceAccount.name }}{{- end }}
