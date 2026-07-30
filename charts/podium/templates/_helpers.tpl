{{- define "podium.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "podium.fullname" -}}
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

{{- define "podium.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "podium.labels" -}}
helm.sh/chart: {{ include "podium.chart" . }}
{{ include "podium.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "podium.selectorLabels" -}}
app.kubernetes.io/name: {{ include "podium.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "podium.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "podium.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "podium.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}

{{- define "podium.secretName" -}}
{{- if .Values.redis.existingSecret.name }}
{{- .Values.redis.existingSecret.name }}
{{- else }}
{{- include "podium.fullname" . }}
{{- end }}
{{- end }}

{{- define "podium.basicAuthSecretName" -}}
{{- if .Values.basicAuth.existingSecret.name }}
{{- .Values.basicAuth.existingSecret.name }}
{{- else }}
{{- include "podium.fullname" . }}
{{- end }}
{{- end }}

{{- define "podium.enrichmentCacheSecretName" -}}
{{- if .Values.config.enrichment.cache.existingSecret.name }}
{{- .Values.config.enrichment.cache.existingSecret.name }}
{{- else }}
{{- include "podium.fullname" . }}
{{- end }}
{{- end }}

{{- define "podium.env" -}}
- name: PODIUM_REDIS_HOST
  value: {{ .Values.redis.host | quote }}
- name: PODIUM_REDIS_PORT
  value: {{ .Values.redis.port | quote }}
- name: PODIUM_REDIS_DB
  value: {{ .Values.redis.db | quote }}
- name: PODIUM_REDIS_CLUSTER_ENABLED
  value: {{ .Values.redis.cluster.enabled | quote }}
{{- if .Values.redis.cluster.addrs }}
- name: PODIUM_REDIS_ADDRS
  value: {{ join "," .Values.redis.cluster.addrs | quote }}
{{- end }}
{{- if or .Values.redis.password .Values.redis.existingSecret.name }}
- name: PODIUM_REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "podium.secretName" . }}
      key: {{ .Values.redis.existingSecret.passwordKey }}
{{- end }}
- name: PODIUM_API_MAXRETURNEDMEMBERS
  value: {{ .Values.config.maxReturnedMembers | quote }}
- name: PODIUM_API_MAXREADBUFFERSIZE
  value: {{ .Values.config.maxReadBufferSize | quote }}
- name: PODIUM_GRACEPERIOD_MS
  value: {{ .Values.config.gracePeriodMilliseconds | quote }}
- name: PODIUM_ENRICHMENT_REQUEST_TIMEOUT
  value: {{ .Values.config.enrichment.requestTimeout | quote }}
- name: PODIUM_ENRICHMENT_CACHE_ADDR
  value: {{ .Values.config.enrichment.cache.addr | quote }}
- name: PODIUM_ENRICHMENT_CACHE_TTL
  value: {{ .Values.config.enrichment.cache.ttl | quote }}
{{- if or .Values.config.enrichment.cache.password .Values.config.enrichment.cache.existingSecret.name }}
- name: PODIUM_ENRICHMENT_CACHE_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "podium.enrichmentCacheSecretName" . }}
      key: {{ .Values.config.enrichment.cache.existingSecret.passwordKey }}
{{- end }}
{{- if .Values.config.enrichment.providers }}
- name: PODIUM_ENRICHMENT_PROVIDERS
  value: {{ .Values.config.enrichment.providers | quote }}
{{- end }}
{{- if .Values.basicAuth.enabled }}
- name: PODIUM_BASICAUTH_USERNAME
  valueFrom:
    secretKeyRef:
      name: {{ include "podium.basicAuthSecretName" . }}
      key: {{ .Values.basicAuth.existingSecret.usernameKey }}
- name: PODIUM_BASICAUTH_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "podium.basicAuthSecretName" . }}
      key: {{ .Values.basicAuth.existingSecret.passwordKey }}
{{- end }}
{{- range $name, $value := .Values.observability.env }}
- name: {{ $name }}
  value: {{ $value | quote }}
{{- end }}
{{- with .Values.extraEnv }}
{{ toYaml . }}
{{- end }}
{{- end }}
