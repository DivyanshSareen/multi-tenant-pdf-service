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

# Start Ollama in the background if the LLM provider is ollama (default).
# Binds to 0.0.0.0 so the Kind cluster can reach it via the host IP.
if [ "${LLM_PROVIDER:-ollama}" = "ollama" ]; then
  if ! pgrep -x ollama > /dev/null; then
    echo "==> Starting Ollama (binding to 0.0.0.0)"
    OLLAMA_HOST=0.0.0.0 ollama serve > /tmp/ollama.log 2>&1 &
    sleep 2
  else
    echo "==> Ollama already running — restarting with OLLAMA_HOST=0.0.0.0"
    pkill -x ollama
    sleep 1
    OLLAMA_HOST=0.0.0.0 ollama serve > /tmp/ollama.log 2>&1 &
    sleep 2
  fi
fi

echo "==> Building Docker image: ${IMAGE_NAME}:${IMAGE_TAG}"
docker build -f deployments/docker/Dockerfile -t "${IMAGE_NAME}:${IMAGE_TAG}" .

echo "==> Loading image into Kind cluster: ${CLUSTER_NAME}"
kind load docker-image "${IMAGE_NAME}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

echo "==> Installing/upgrading Helm release: ${HELM_RELEASE}"

# Detect host IP so Ollama (running on the Mac) is reachable from inside the Kind cluster.
HOST_IP=$(ipconfig getifaddr en0 2>/dev/null || ifconfig en0 | awk '/inet /{print $2}')

helm upgrade --install "${HELM_RELEASE}" "${HELM_CHART}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set llm.openaiApiKey="${OPENAI_API_KEY:-}" \
  --set llm.provider="${LLM_PROVIDER:-ollama}" \
  --set llm.ollamaModel="${OLLAMA_MODEL:-llama3.1}" \
  --set llm.ollamaBaseUrl="http://${HOST_IP}:11434" \
  --wait --timeout 5m

echo "==> Deployment complete."
echo "    Service available at: http://localhost:30080"
echo "    Test with: ./scripts/test-api.sh"
