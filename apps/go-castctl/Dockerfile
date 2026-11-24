# setup project and deps
FROM golang:1.27.4-trixie AS init

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
# Rebuild Castor v2.5.0 with the fix for CVE-2026-78661.
FROM golang:1.27.4-trixie AS castor-build
RUN apt-get update && \
    apt-get install -y --no-install-recommends cmake && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /src/castor
# This commit is the source for the pinned upstream runtime image.
RUN git init . && \
    git remote add origin https://github.com/stupside/castor.git && \
    git fetch --depth 1 origin 00ce169dc09004e4cdecf4b86d3aa99523d32701 && \
    git checkout --detach FETCH_HEAD && \
    git submodule update --init --depth 1
RUN go get golang.org/x/net@v0.62.0
# Retain upstream's Whisper support. Disable tuning for the build host CPU.
RUN cmake -S third_party/whisper.cpp -B third_party/whisper.cpp/build_go \
      -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF \
      -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=OFF -DGGML_NATIVE=OFF && \
    cmake --build third_party/whisper.cpp/build_go --target whisper --parallel 2
RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/src/castor/third_party/whisper.cpp/include -I/src/castor/third_party/whisper.cpp/ggml/include" \
    CGO_LDFLAGS="-L/src/castor/third_party/whisper.cpp/build_go/src -L/src/castor/third_party/whisper.cpp/build_go/ggml/src" \
    go build -trimpath -ldflags="-s -w -X github.com/stupside/castor/cmd/internal/process.version=v2.5.0-monogo" \
      -o /out/castor ./cmd/castor

FROM ghcr.io/stupside/castor:v2.5.0 AS app-runtime
COPY --from=castor-build /out/castor /usr/local/bin/castor

FROM app-runtime
# Install or update runtime packages from the base distribution repositories.
RUN apt-get update && \
    apt-get install -y --no-install-recommends chromium chromium-common libbrotli1 apt libapt-pkg7.0 libgssapi-krb5-2 libk5crypto3 libkrb5-3 libkrb5support0 libminizip1t64 zlib1g && \
    rm -rf /var/lib/apt/lists/*
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
