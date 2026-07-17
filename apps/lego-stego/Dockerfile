# setup project and deps
FROM golang:1.26-trixie AS init

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

WORKDIR /workspace/

COPY go.mod* go.sum* ./
RUN go mod download

# Copy only the selected app and shared packages so unrelated apps stay out of the build context
COPY apps/lego-stego ./apps/lego-stego
COPY pkg ./pkg

FROM init AS vet
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go vet ./apps/lego-stego/... ./pkg/...

# run tests
FROM init AS test
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go test -coverprofile c.out -v ./apps/lego-stego/... ./pkg/... && \
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

RUN LDFLAGS="-s -w -X github.com/toozej/monogo/pkg/version.Version=${VERSION} -X github.com/toozej/monogo/pkg/version.Commit=${COMMIT} -X github.com/toozej/monogo/pkg/version.Branch=${BRANCH} -X github.com/toozej/monogo/pkg/version.BuiltAt=${BUILT_AT} -X github.com/toozej/monogo/pkg/version.Builder=${BUILDER}" && \
    CGO_ENABLED=0 go build -o /out/lego-stego -ldflags="${LDFLAGS}" ./apps/lego-stego

# runtime image
FROM scratch
# Copy our executable.
COPY --from=build /out/lego-stego /go/bin/lego-stego
# Include the project license in the runtime image.
COPY LICENSE /usr/share/doc/lego-stego/copyright
# Expose port for publishing as web service
# EXPOSE 8081
# Run the binary.
USER 65532:65532
ENTRYPOINT ["/go/bin/lego-stego"]
