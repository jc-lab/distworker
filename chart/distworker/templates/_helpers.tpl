{{- define "distworker.credentials-secret-name" -}}
{{- printf "%s-credentials" (include "common.names.fullname" .) -}}
{{- end }}

{{- define "distworker.config-map-name" -}}
{{- printf "%s-config" (include "common.names.fullname" .) -}}
{{- end }}

{{- define "distworker.controller.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.controller.image "global" .Values.global "chart" .Chart) }}
{{- end -}}
{{- define "distworker.controller.imagePullSecrets" -}}
{{- include "common.images.pullSecrets" (dict "images" (list .Values.controller.image) "global" .Values.global) -}}
{{- end -}}

{{- define "distworker.get-external-base-url" -}}
{{- $zeroback := .Values.global.zeroback -}}
{{- if $zeroback -}}
{{- $zeroback.externalBaseUrl -}}
{{- end -}}
{{- end -}}

{{- define "distworker.secret.get-or-set" -}}
{{- $password := "" }}
{{- $chartName := default "" .chartName }}
{{- $secretData := (lookup "v1" "Secret" $.context.Release.Namespace .secret).data }}
{{- if $secretData }}
  {{- if hasKey $secretData .key }}
    {{- $password = index $secretData .key | b64dec }}
  {{- else }}
    {{- printf "\nPASSWORDS ERROR: The secret \"%s\" does not contain the key \"%s\"\n" .secret .key | fail -}}
  {{- end -}}
{{- else }}
  {{- $password = .providedValue }}
{{- end }}
{{- $password | b64enc -}}
{{- end -}}


{{- define "distworker.simple-kms.envs" -}}
{{- $credentialSecretName := include "distworker.credentials-secret-name" . -}}
- name: "APP_SIMPLE_KMS_PASSWORD"
  valueFrom:
    secretKeyRef:
      name: {{ $credentialSecretName | quote }}
      key: simple-kms-password
- name: "APP_SIMPLE_KMS_WRAP_KEY"
  valueFrom:
    secretKeyRef:
      name: {{ $credentialSecretName | quote }}
      key: simple-kms-wrap-key
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "distworker.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "common.names.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Return true if cert-manager required annotations for TLS signed certificates are set in the Ingress annotations
Ref: https://cert-manager.io/docs/usage/ingress/#supported-annotations
*/}}
{{- define "distworker.ingress.certManagerRequest" -}}
{{ if or (hasKey . "cert-manager.io/cluster-issuer") (hasKey . "cert-manager.io/issuer") }}
    {{- true -}}
{{- end -}}
{{- end -}}

{{/*
Compile all warnings into a single message.
*/}}
{{- define "distworker.validateValues" -}}
{{- $messages := list -}}
{{- $messages := append $messages (include "distworker.validateValues.foo" .) -}}
{{- $messages := append $messages (include "distworker.validateValues.bar" .) -}}
{{- $messages := without $messages "" -}}
{{- $message := join "\n" $messages -}}

{{- if $message -}}
{{-   printf "\nVALUES VALIDATION:\n%s" $message -}}
{{- end -}}
{{- end -}}
