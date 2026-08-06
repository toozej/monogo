# setup project and deps
FROM golang:1.26-trixie AS init

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

WORKDIR /workspace/

COPY go.mod* go.sum* ./
RUN go mod download

# Copy only the selected app and shared packages so unrelated apps stay out of the build context
COPY apps/go-castctl ./apps/go-castctl
COPY pkg ./pkg

FROM init AS vet
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go vet ./apps/go-castctl/... ./pkg/...

# run tests
FROM init AS test
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o apps/go-castctl/internal/server/web/app.wasm ./apps/go-castctl/web && \
    go test -coverprofile c.out -v ./apps/go-castctl/... ./pkg/... && \
    echo "Statements missing coverage" && \
    grep -v -e " 1$" c.out

# build binary
FROM init AS build
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG VERSION=unknown
ARG COMMIT=unknown
ARG BRANCH=unknown
ARG BUILT_AT=unknown
ARG BUILDER=unknown

# Build the go-app browser client before the native binary so it can be embedded.
RUN GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o apps/go-castctl/internal/server/web/app.wasm ./apps/go-castctl/web && \
    LDFLAGS="-s -w -X github.com/toozej/monogo/pkg/version.Version=${VERSION} -X github.com/toozej/monogo/pkg/version.Commit=${COMMIT} -X github.com/toozej/monogo/pkg/version.Branch=${BRANCH} -X github.com/toozej/monogo/pkg/version.BuiltAt=${BUILT_AT} -X github.com/toozej/monogo/pkg/version.Builder=${BUILDER}" && \
    CGO_ENABLED=0 go build -o /out/go-castctl -ldflags="${LDFLAGS}" ./apps/go-castctl

# hadolint ignore=DL3059
RUN mkdir -p /container-root/tmp && \
    chown -R 65532:65532 /container-root

# runtime image
FROM ghcr.io/stupside/castor:v1.8.1
# Copy our executable.
COPY --from=build /out/go-castctl /go/bin/go-castctl
# Include the project license in the runtime image.
COPY LICENSE /usr/share/doc/go-castctl/copyright
COPY --from=build --chown=65532:65532 /container-root/tmp /tmp
WORKDIR /tmp
# Expose port for publishing as web service
EXPOSE 8080
# Verify that the application binary can start without requiring a shell or
# additional utilities in the minimal runtime image.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/go/bin/go-castctl", "version"]
# Run the binary.
USER 65532:65532
ENTRYPOINT ["/go/bin/go-castctl"]
