#!/usr/bin/env bash
set -Eeuo pipefail

# Build and deploy the platform to the three-node K3s lab.
# Secrets remain outside the repository under STATE_DIR.

REPO_DIR="${REPO_DIR:-$HOME/apps/k8s-runtime-incident-platform}"
STATE_DIR="${STATE_DIR:-$HOME/k3s-relate/deploy-state}"
NAMESPACE="${NAMESPACE:-incident-system}"
SERVER_IP="${SERVER_IP:-192.168.31.44}"
WORKER_USER="${WORKER_USER:-lcd}"
WORKER_IPS=(192.168.31.205 192.168.31.191)
NODE_PORT="${NODE_PORT:-30081}"
IMAGE_TAG="${IMAGE_TAG:-}"
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmmirror.com}"
VALUES_FILE="$STATE_DIR/krip-values.yaml"
CERT_DIR="$STATE_DIR/webhook-certs"
CHART_DIR="$REPO_DIR/deploy/helm/runtime-incident-platform"
SSH_OPTIONS=(-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new)
USE_SSHPASS=0
WORKER_PASSWORD=""

cleanup() {
  unset SSHPASS WORKER_PASSWORD PG_PASSWORD PG_PASSWORD_ENCODED CA_BUNDLE WEBHOOK_SECRET
}
trap cleanup EXIT

log() { printf '\n==> %s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

require_tool() {
  command -v "$1" >/dev/null 2>&1 || die "required tool is missing: $1"
}

ensure_worker_password() {
  if [[ -n "$WORKER_PASSWORD" ]]; then
    return
  fi
  read -rsp "Worker SSH/sudo password (not stored): " WORKER_PASSWORD
  printf '\n'
  [[ -n "$WORKER_PASSWORD" ]] || die "worker password cannot be empty"
  export SSHPASS="$WORKER_PASSWORD"
}

ssh_exec() {
  local host="$1"
  shift
  if (( USE_SSHPASS )); then
    sshpass -e ssh "${SSH_OPTIONS[@]}" "$WORKER_USER@$host" "$@"
  else
    ssh "${SSH_OPTIONS[@]}" "$WORKER_USER@$host" "$@"
  fi
}

scp_to_worker() {
  local source="$1" host="$2" destination="$3"
  if (( USE_SSHPASS )); then
    sshpass -e scp "${SSH_OPTIONS[@]}" "$source" "$WORKER_USER@$host:$destination"
  else
    scp "${SSH_OPTIONS[@]}" "$source" "$WORKER_USER@$host:$destination"
  fi
}

remote_sudo() {
  local host="$1"
  shift
  if ssh_exec "$host" "sudo -n true" >/dev/null 2>&1; then
    ssh_exec "$host" "sudo -n $*"
    return
  fi
  ensure_worker_password
  printf '%s\n' "$WORKER_PASSWORD" | ssh_exec "$host" "sudo -S -p '' $*"
}

for tool in git docker kubectl helm openssl base64 python3 ssh scp curl; do
  require_tool "$tool"
done

[[ -d "$REPO_DIR/.git" ]] || die "repository not found at $REPO_DIR; clone it from GitHub first"
mkdir -p "$STATE_DIR" "$CERT_DIR"
chmod 700 "$STATE_DIR" "$CERT_DIR"

log "Checking local repository and pulling GitHub changes"
if [[ -n "$(git -C "$REPO_DIR" status --porcelain)" ]]; then
  die "repository has local changes; commit or stash them before deployment"
fi
git -C "$REPO_DIR" pull --ff-only
IMAGE_TAG="${IMAGE_TAG:-$(git -C "$REPO_DIR" rev-parse --short=12 HEAD)}"
BACKEND_IMAGE="docker.io/library/runtime-incident-platform-backend:${IMAGE_TAG}"
FRONTEND_IMAGE="docker.io/library/runtime-incident-platform-frontend:${IMAGE_TAG}"
IMAGE_ARCHIVE="$STATE_DIR/krip-images-${IMAGE_TAG}.tar"

grep -q 'verbs: \[get, list, watch, patch, update\]' "$CHART_DIR/templates/rbac.yaml" || \
  die "Helm RBAC does not include patch/update for observability resources"

log "Checking cluster prerequisites"
sudo -v
kubectl get node k3s-server >/dev/null
kubectl -n "$NAMESPACE" get pod postgresql-0 >/dev/null
kubectl -n "$NAMESPACE" get service postgresql >/dev/null
kubectl -n "$NAMESPACE" rollout status deployment/redis --timeout=120s
kubectl -n monitoring get service monitoring-kube-prometheus-prometheus >/dev/null
kubectl -n "$NAMESPACE" get secret postgres-credentials >/dev/null

log "Checking worker SSH authentication"
for host in "${WORKER_IPS[@]}"; do
  if ! ssh -o BatchMode=yes "${SSH_OPTIONS[@]}" "$WORKER_USER@$host" true >/dev/null 2>&1; then
    USE_SSHPASS=1
    require_tool sshpass
    ensure_worker_password
    break
  fi
done
for host in "${WORKER_IPS[@]}"; do
  ssh_exec "$host" true
done

log "Building backend image"
docker build --progress=plain \
  --build-arg "GOPROXY=$GOPROXY" \
  --build-arg "HTTP_PROXY=${HTTP_PROXY:-}" \
  --build-arg "HTTPS_PROXY=${HTTPS_PROXY:-}" \
  --build-arg "NO_PROXY=${NO_PROXY:-}" \
  -t "$BACKEND_IMAGE" "$REPO_DIR"

log "Building frontend image"
docker build --progress=plain \
  --build-arg "NPM_REGISTRY=$NPM_REGISTRY" \
  --build-arg "HTTP_PROXY=${HTTP_PROXY:-}" \
  --build-arg "HTTPS_PROXY=${HTTPS_PROXY:-}" \
  --build-arg "NO_PROXY=${NO_PROXY:-}" \
  -t "$FRONTEND_IMAGE" "$REPO_DIR/runtime-console"

log "Exporting and importing images on k3s-server"
docker save -o "$IMAGE_ARCHIVE" "$BACKEND_IMAGE" "$FRONTEND_IMAGE"
sudo k3s ctr -n k8s.io images import "$IMAGE_ARCHIVE"

log "Distributing the same images to workers"
for host in "${WORKER_IPS[@]}"; do
  printf '  -> %s\n' "$host"
  scp_to_worker "$IMAGE_ARCHIVE" "$host" "/tmp/$(basename "$IMAGE_ARCHIVE")"
  remote_sudo "$host" "k3s ctr -n k8s.io images import /tmp/$(basename "$IMAGE_ARCHIVE")"
done

log "Creating the database connection Secret"
PG_PASSWORD="$(kubectl -n "$NAMESPACE" get secret postgres-credentials -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)"
PG_PASSWORD_ENCODED="$(printf '%s' "$PG_PASSWORD" | python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.stdin.read(), safe=""))')"
kubectl -n "$NAMESPACE" create secret generic runtime-incident-platform-database \
  --from-literal=dsn="postgres://incident:${PG_PASSWORD_ENCODED}@postgresql.${NAMESPACE}.svc.cluster.local:5432/incident?sslmode=disable" \
  --dry-run=client -o yaml | kubectl apply -f -
unset PG_PASSWORD PG_PASSWORD_ENCODED

log "Creating Admission Webhook certificates and Secrets"
if [[ ! -s "$CERT_DIR/tls.crt" || ! -s "$CERT_DIR/tls.key" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -subj "/CN=runtime-incident-platform-backend.${NAMESPACE}.svc" \
    -addext "subjectAltName=DNS:runtime-incident-platform-backend.${NAMESPACE}.svc,DNS:runtime-incident-platform-backend.${NAMESPACE}.svc.cluster.local" \
    -keyout "$CERT_DIR/tls.key" -out "$CERT_DIR/tls.crt"
  chmod 600 "$CERT_DIR/tls.key" "$CERT_DIR/tls.crt"
fi
kubectl -n "$NAMESPACE" create secret tls runtime-incident-platform-webhook-tls \
  --key="$CERT_DIR/tls.key" --cert="$CERT_DIR/tls.crt" \
  --dry-run=client -o yaml | kubectl apply -f -
if ! kubectl -n "$NAMESPACE" get secret runtime-incident-platform-webhook >/dev/null 2>&1; then
  WEBHOOK_SECRET="$(openssl rand -hex 32)"
  kubectl -n "$NAMESPACE" create secret generic runtime-incident-platform-webhook \
    --from-literal=secret="$WEBHOOK_SECRET"
  unset WEBHOOK_SECRET
fi

log "Writing lab-specific Helm values outside the repository"
cat >"$VALUES_FILE" <<EOF
backend:
  replicaCount: 1
  image:
    repository: docker.io/library/runtime-incident-platform-backend
    tag: ${IMAGE_TAG}
    pullPolicy: IfNotPresent
frontend:
  replicaCount: 1
  image:
    repository: docker.io/library/runtime-incident-platform-frontend
    tag: ${IMAGE_TAG}
    pullPolicy: IfNotPresent
  service:
    type: NodePort
    port: 80
    nodePort: ${NODE_PORT}
config:
  clusterId: k3s-lab
  kubernetesEnabled: true
  redisAddr: redis.${NAMESPACE}.svc.cluster.local:6379
  prometheusUrl: http://monitoring-kube-prometheus-prometheus.monitoring.svc.cluster.local:9090
  lokiUrl: http://loki.monitoring.svc.cluster.local:3100
  tempoUrl: http://tempo.monitoring.svc.cluster.local:3200
postgres:
  existingSecret: runtime-incident-platform-database
  dsnKey: dsn
webhook:
  enabled: true
  existingSecret: runtime-incident-platform-webhook
  secretKey: secret
  tlsSecret: runtime-incident-platform-webhook-tls
  url: ""
  allowlist: ""
  targetName: incident-agent
serviceMonitor:
  enabled: false
EOF
chmod 600 "$VALUES_FILE"

log "Rendering and deploying the Helm release"
CA_BUNDLE="$(base64 -w0 "$CERT_DIR/tls.crt")"
helm lint "$CHART_DIR" -f "$VALUES_FILE" --set-string "webhook.caBundle=$CA_BUNDLE"
helm template krip "$CHART_DIR" -n "$NAMESPACE" -f "$VALUES_FILE" \
  --set-string "webhook.caBundle=$CA_BUNDLE" >/dev/null
# Remove the one-off Service used before NodePort support moved into the Chart.
kubectl -n "$NAMESPACE" delete service runtime-incident-platform-nodeport --ignore-not-found
helm upgrade --install krip "$CHART_DIR" \
  --namespace "$NAMESPACE" \
  --values "$VALUES_FILE" \
  --set-string "webhook.caBundle=$CA_BUNDLE" \
  --wait --timeout 10m
unset CA_BUNDLE

log "Verifying workloads and the fixed NodePort"
kubectl -n "$NAMESPACE" rollout status deployment/runtime-incident-platform-backend --timeout=180s
kubectl -n "$NAMESPACE" rollout status deployment/runtime-incident-platform-frontend --timeout=180s
kubectl -n "$NAMESPACE" get pods -o wide
kubectl -n "$NAMESPACE" get service runtime-incident-platform-frontend
curl --fail --show-error --retry 12 --retry-delay 2 --max-time 10 \
  "http://${SERVER_IP}:${NODE_PORT}/" >/dev/null

printf '\nDeployment complete: http://%s:%s/\n' "$SERVER_IP" "$NODE_PORT"
