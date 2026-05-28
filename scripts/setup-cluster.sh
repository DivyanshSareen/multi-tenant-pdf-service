#!/usr/bin/env bash
# Creates a local Kind cluster named "pdf-service" and verifies it's ready.
set -euo pipefail

CLUSTER_NAME="pdf-service"

# Kind config that maps host port 30080 → NodePort 30080 so the service is
# reachable at http://localhost:30080 without a separate port-forward.
KIND_CONFIG=$(cat <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30080
        hostPort: 30080
        protocol: TCP
EOF
)

echo "==> Creating Kind cluster: ${CLUSTER_NAME}"
echo "${KIND_CONFIG}" | kind create cluster --name "${CLUSTER_NAME}" --config -

echo "==> Verifying cluster nodes are ready"
kubectl wait --for=condition=Ready nodes --all --timeout=60s

echo "==> Cluster context set to: kind-${CLUSTER_NAME}"
kubectl config use-context "kind-${CLUSTER_NAME}"

echo "==> Cluster ready. Run ./scripts/deploy.sh to deploy the service."
