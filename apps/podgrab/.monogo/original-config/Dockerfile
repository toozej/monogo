# setup project and deps
FROM golang:1.26-trixie AS init

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

WORKDIR /go/podgrab/

COPY go.mod* go.sum* ./
RUN go mod download

COPY . ./

FROM init AS vet
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go vet ./...

# run tests
FROM init AS test
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go test -coverprofile c.out -v ./... && \
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

RUN PKG=$(head -n 1 go.mod | cut -c 8-) && \
    LDFLAGS="-s -w -X ${PKG}/pkg/version.Version=${VERSION} -X ${PKG}/pkg/version.Commit=${COMMIT} -X ${PKG}/pkg/version.Branch=${BRANCH} -X ${PKG}/pkg/version.BuiltAt=${BUILT_AT} -X ${PKG}/pkg/version.Builder=${BUILDER}" && \
    CGO_ENABLED=0 go build -ldflags="${LDFLAGS}"

# Install coreutils for sleep and other utilities utilized in devcontainer
RUN apt-get update && apt-get install --no-install-recommends -y coreutils

# runtime image
FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /go/bin/
# Copy our static executable.
COPY --from=build /go/podgrab/podgrab /go/bin/podgrab
# Expose port for publishing as web service
EXPOSE 8080
# Setup volumes for data
ENV CONFIG=/config
ENV DATA=/assets
ENV UID=998
ENV PID=100
ENV GIN_MODE=release
VOLUME ["/config", "/assets"]

USER nonroot
# Run the binary.
ENTRYPOINT ["/go/bin/podgrab"]
