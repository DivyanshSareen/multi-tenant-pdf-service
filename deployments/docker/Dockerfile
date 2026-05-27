# --- Stage 1: build ---
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Download dependencies first — cached as long as go.mod/go.sum don't change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a fully static binary that runs on alpine without glibc.
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# --- Stage 2: runtime ---
FROM alpine:3.19

# ca-certificates is required for HTTPS calls to OpenAI / external services.
# poppler-utils provides pdftotext and pdfinfo for PDF text extraction.
RUN apk add --no-cache ca-certificates poppler-utils

WORKDIR /app

COPY --from=builder /server /server

EXPOSE 8080

CMD ["/server"]
