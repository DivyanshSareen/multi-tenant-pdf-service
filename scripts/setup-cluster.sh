#!/usr/bin/env bash
# Creates a local Kind cluster named "pdf-service" and verifies it's ready.
set -euo pipefail

CLUSTER_NAME="pdf-service"

echo "==> Creating Kind cluster: ${CLUSTER_NAME}"
kind create cluster --name "${CLUSTER_NAME}"

echo "==> Verifying cluster nodes are ready"
kubectl wait --for=condition=Ready nodes --all --timeout=60s

echo "==> Cluster context set to: kind-${CLUSTER_NAME}"
kubectl config use-context "kind-${CLUSTER_NAME}"

echo "==> Cluster ready. Run ./scripts/deploy.sh to deploy the service."
