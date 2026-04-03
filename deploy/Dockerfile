FROM golang:1.26.1-alpine3.22 AS builder
RUN apk add --no-cache ca-certificates git tzdata
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ARG VERSION=1.0.0
ARG BUILD_TIME
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X 'main.Version=${VERSION}'" -o /build/zs-fhir-validator ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot
USER nonroot:nonroot
COPY --from=builder --chown=nonroot:nonroot /build/zs-fhir-validator /app/zs-fhir-validator
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /app
EXPOSE 8086
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 CMD ["/app/zs-fhir-validator","healthcheck"]
ENTRYPOINT ["/app/zs-fhir-validator"]
LABEL org.opencontainers.image.title="zs-fhir-validator"
LABEL org.opencontainers.image.vendor="ZarishSphere"
LABEL org.opencontainers.image.licenses="Apache-2.0"
