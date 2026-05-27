#!/usr/bin/env bash
# Builds the Docker image, loads it into the Kind cluster, and installs the Helm chart.
set -euo pipefail

CLUSTER_NAME="pdf-service"
IMAGE_NAME="pdf-service"
IMAGE_TAG="latest"
HELM_RELEASE="pdf-service"
HELM_CHART="deployments/helm/pdf-service"
NAMESPACE="pdf-service"

# Navigate to repo root regardless of where the script is called from.
cd "$(dirname "$0")/.."

echo "==> Building Docker image: ${IMAGE_NAME}:${IMAGE_TAG}"
docker build -f deployments/docker/Dockerfile -t "${IMAGE_NAME}:${IMAGE_TAG}" .

echo "==> Loading image into Kind cluster: ${CLUSTER_NAME}"
kind load docker-image "${IMAGE_NAME}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

echo "==> Installing/upgrading Helm release: ${HELM_RELEASE}"
helm upgrade --install "${HELM_RELEASE}" "${HELM_CHART}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set llm.openaiApiKey="${OPENAI_API_KEY:-}" \
  --wait --timeout 5m

echo "==> Deployment complete."
echo "    Service available at: http://localhost:30080"
echo "    Test with: ./scripts/test-api.sh"
