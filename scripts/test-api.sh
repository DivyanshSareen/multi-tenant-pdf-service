#!/usr/bin/env bash
# Smoke-tests all API endpoints. Requires a running service and a sample PDF.
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:30080}"
API_KEY="${API_KEY:-changeme}"
AUTH_HEADER="Authorization: Bearer ${API_KEY}"

# Create a minimal valid PDF for testing if none exists.
SAMPLE_PDF="/tmp/sample-test.pdf"
if [ ! -f "${SAMPLE_PDF}" ]; then
  echo "==> Creating minimal sample PDF at ${SAMPLE_PDF}"
  # Minimal 1-page PDF (raw bytes — works without any external tool).
  printf '%s' '%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Contents 4 0 R/Resources<</Font<</F1<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>>>>>>>endobj
4 0 obj<</Length 44>>stream
BT /F1 12 Tf 100 700 Td (Hello World PDF) Tj ET
endstream endobj
xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000274 00000 n
trailer<</Size 5/Root 1 0 R>>
startxref
368
%%EOF' > "${SAMPLE_PDF}"
fi

echo ""
echo "=============================="
echo " Multi-Tenant PDF Service Tests"
echo " Base URL: ${BASE_URL}"
echo "=============================="

# 1. Health check (no auth needed)
echo ""
echo "--> GET /health"
curl -sf "${BASE_URL}/health" | python3 -m json.tool

# 2. Readiness check (no auth needed)
echo ""
echo "--> GET /ready"
curl -sf "${BASE_URL}/ready" | python3 -m json.tool

# 3. Upload PDF for tenant "acme"
echo ""
echo "--> POST /api/v1/upload (tenant: acme)"
UPLOAD_RESP=$(curl -sf -X POST "${BASE_URL}/api/v1/upload" \
  -H "${AUTH_HEADER}" \
  -F "tenantName=acme" \
  -F "file=@${SAMPLE_PDF};type=application/pdf")
echo "${UPLOAD_RESP}" | python3 -m json.tool
DOC_ID=$(echo "${UPLOAD_RESP}" | python3 -c "import sys,json; print(json.load(sys.stdin)['document_id'])" 2>/dev/null || echo "")

# 4. Upload second PDF for different tenant "globex"
echo ""
echo "--> POST /api/v1/upload (tenant: globex)"
curl -sf -X POST "${BASE_URL}/api/v1/upload" \
  -H "${AUTH_HEADER}" \
  -F "tenantName=globex" \
  -F "file=@${SAMPLE_PDF};type=application/pdf" | python3 -m json.tool

# 5. List all tenants
echo ""
echo "--> GET /api/v1/tenants"
curl -sf "${BASE_URL}/api/v1/tenants" \
  -H "${AUTH_HEADER}" | python3 -m json.tool

# 6. Get single tenant
echo ""
echo "--> GET /api/v1/tenants/acme"
curl -sf "${BASE_URL}/api/v1/tenants/acme" \
  -H "${AUTH_HEADER}" | python3 -m json.tool

# 7. Get tenant documents
echo ""
echo "--> GET /api/v1/tenants/acme/documents"
curl -sf "${BASE_URL}/api/v1/tenants/acme/documents" \
  -H "${AUTH_HEADER}" | python3 -m json.tool

# 8. Test auth rejection
echo ""
echo "--> GET /api/v1/tenants (no auth — expect 401)"
curl -s -o /dev/null -w "HTTP %{http_code}\n" "${BASE_URL}/api/v1/tenants"

# 9. Delete tenant "globex"
echo ""
echo "--> DELETE /api/v1/tenants/globex"
curl -sf -X DELETE "${BASE_URL}/api/v1/tenants/globex" \
  -H "${AUTH_HEADER}" | python3 -m json.tool

echo ""
echo "=============================="
echo " All tests passed!"
echo "=============================="
