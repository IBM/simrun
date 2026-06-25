# syntax=docker/dockerfile:1

# --- Build stage ---
FROM golang:1.25.11 AS builder

WORKDIR /build

# Node.js is needed to build the SvelteKit frontend
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs

COPY . .

# Build the frontend
RUN --mount=type=cache,target=/root/.npm \
    cd web/frontend && npm ci && npm run build

# Copy built frontend into the Go embed directory
RUN rm -rf internal/web/frontend && \
    mkdir -p internal/web/frontend && \
    cp -r web/frontend/build/* internal/web/frontend/

# Build the server binary with the embedded frontend
ARG version=unknown
RUN git config --global --add safe.directory /build && \
    CGO_ENABLED=0 go build \
    -ldflags="-w -s \
      -X github.com/IBM/simrun/internal/version.Version=${version} \
      -X github.com/IBM/simrun/internal/version.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) \
      -X github.com/IBM/simrun/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /simrun-server cmd/simrun/main.go

# --- Runtime stage ---
FROM alpine:3.21

RUN apk add --no-cache \
    ca-certificates \
    git \
    openssh-client \
    curl \
    aws-cli \
    python3 \
    py3-pip

# Google Cloud SDK (https://cloud.google.com/sdk/docs/install-sdk#linux)
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
      arm64) GCLOUD_ARCH=arm ;; \
      *)     GCLOUD_ARCH=x86_64 ;; \
    esac && \
    curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-${GCLOUD_ARCH}.tar.gz && \
    tar -xf google-cloud-cli-linux-${GCLOUD_ARCH}.tar.gz && \
    ./google-cloud-sdk/install.sh --quiet --path-update=false && \
    rm google-cloud-cli-linux-${GCLOUD_ARCH}.tar.gz
ENV PATH="/google-cloud-sdk/bin:${PATH}"

# Azure CLI
RUN pip3 install --no-cache-dir --break-system-packages azure-cli

# Run as a non-root user
RUN addgroup -S nonroot && adduser -S -G nonroot nonroot

COPY --from=builder /simrun-server /simrun-server

USER nonroot
WORKDIR /
ENTRYPOINT ["/simrun-server"]
