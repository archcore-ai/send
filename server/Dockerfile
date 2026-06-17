# syntax=docker/dockerfile:1
#
# Build a static, cgo-free sendd binary, then ship it on distroless. No age, no
# shell, no package manager in the final image — a minimal attack surface for a
# zero-knowledge backend.

FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/sendd .
# Pre-create the data dir owned by the distroless nonroot uid (65532). A fresh
# named volume mounted at /data inherits this ownership, so sendd can write
# SQLite + blobs without a chown init.
RUN mkdir -p /data && chown -R 65532:65532 /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sendd /usr/local/bin/sendd
COPY --from=build --chown=65532:65532 /data /data
USER 65532:65532
ENV SEND_LISTEN=:8080 \
    SEND_DB_PATH=/data/sends.db \
    SEND_BLOB_DIR=/data/blobs
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/sendd"]
