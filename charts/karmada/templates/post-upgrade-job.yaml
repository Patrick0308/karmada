{{- $name := include "karmada.name" . -}}
{{- $namespace := include "karmada.namespace" . -}}
{{- if eq .Values.installMode "host" }}
apiVersion: batch/v1
kind: Job
metadata:
  name: "{{ $name }}-post-upgrade"
  namespace: {{ $namespace }}
  annotations:
    # This is what defines this resource as a hook. Without this line, the
    # job is considered part of the release.
    "helm.sh/hook": post-upgrade
    "helm.sh/hook-weight": "3"
    "helm.sh/hook-delete-policy": {{ .Values.postUpgradeJob.hookDeletePolicy }}
  {{- if "karmada.postUpgradeJob.labels" }}
  labels:
    {{- include "karmada.postUpgradeJob.labels" . | nindent 4 }}
  {{- end }}
spec:
  parallelism: 1
  completions: 1
  template:
    metadata:
      name: {{ $name }}-post-upgrade
      labels:
        app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
        app.kubernetes.io/instance: {{ $name | quote }}
        helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version }}"
    spec:
      {{- with .Values.postUpgradeJob.tolerations}}
      tolerations:
      {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.postUpgradeJob.nodeSelector }}
      nodeSelector:
      {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ $name }}-post-upgrade
      restartPolicy: Never
      containers:
      - name: post-upgrade
        image: {{ template "karmada.kubectl.image" . }}
        imagePullPolicy: {{ .Values.kubectl.image.pullPolicy }}
        workingDir: /opt/mount
        command:
        - /bin/sh
        - -c
        - |
          bash <<'EOF'
          set -ex
          kubectl apply -f /static-resources --kubeconfig /etc/kubeconfig
          EOF
        volumeMounts:
        - name: {{ $name }}-upgrade-static-resources
          mountPath: /static-resources
        {{ include "karmada.kubeconfig.volumeMount" . | nindent 8 }}
      volumes:
      - name: {{ $name }}-upgrade-static-resources
        configMap:
          name: {{ $name }}-upgrade-static-resources
      {{ include "karmada.kubeconfig.volume" . | nindent 6 }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ $name }}-post-upgrade
  namespace: {{ $namespace }}
  annotations:
    "helm.sh/hook": post-upgrade
    "helm.sh/hook-weight": "1"
  {{- if "karmada.postUpgradeJob.labels" }}
  labels:
    {{- include "karmada.postUpgradeJob.labels" . | nindent 4 }}
  {{- end }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ $name }}-post-upgrade
  annotations:
    "helm.sh/hook": post-upgrade
    "helm.sh/hook-weight": "1"
  {{- if "karmada.postUpgradeJob.labels" }}
  labels:
    {{- include "karmada.postUpgradeJob.labels" . | nindent 4 }}
  {{- end }}
rules:
  - apiGroups: ['*']
    resources: ['*']
    verbs: ["get", "watch", "list", "create", "update", "patch", "delete"]
  - nonResourceURLs: ['*']
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ $name }}-post-upgrade
  annotations:
    "helm.sh/hook": post-upgrade
    "helm.sh/hook-weight": "1"
  {{- if "karmada.postUpgradeJob.labels" }}
  labels:
    {{- include "karmada.postUpgradeJob.labels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ $name }}-post-upgrade
subjects:
  - kind: ServiceAccount
    name: {{ $name }}-post-upgrade
    namespace: {{ $namespace }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ $name }}-upgrade-static-resources
  namespace: {{ $namespace }}
  annotations:
    "helm.sh/hook": post-upgrade
    "helm.sh/hook-weight": "1"
  {{- if "karmada.postUpgradeJob.labels" }}
  labels:
    {{- include "karmada.postUpgradeJob.labels" . | nindent 4 }}
  {{- end }}
data:
  {{- print "static-resources.yaml: " | nindent 6 }} |-
    {{- include "karmada.post-upgrade.configuration" . | nindent 8 }}
---
{{- end }}
