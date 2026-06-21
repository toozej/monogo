# setup project and deps
FROM golang:1.26-trixie AS init

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

WORKDIR /go/terranotate/

COPY go.mod* go.sum* ./
RUN go mod download

COPY . ./

FROM init AS vet
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go vet ./...

# run tests
FROM init AS test
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go test -coverprofile c.out -v ./...

# build binary
FROM init AS build
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG VERSION=unknown
ARG COMMIT=unknown
ARG BRANCH=unknown
ARG BUILT_AT=unknown
ARG BUILDER=unknown

# Install coreutils for sleep and other utilities utilized in devcontainer
RUN apt-get update && apt-get install --no-install-recommends -y coreutils

RUN PKG=$(head -n 1 go.mod | cut -c 8-) && \
    LDFLAGS="-s -w -X ${PKG}/pkg/version.Version=${VERSION} -X ${PKG}/pkg/version.Commit=${COMMIT} -X ${PKG}/pkg/version.Branch=${BRANCH} -X ${PKG}/pkg/version.BuiltAt=${BUILT_AT} -X ${PKG}/pkg/version.Builder=${BUILDER}" && \
    CGO_ENABLED=0 go build -ldflags="${LDFLAGS}"

# runtime image
FROM scratch
# Copy our static executable.
COPY --from=build /go/terranotate/terranotate /go/bin/terranotate
# Run the binary.
USER nonroot
ENTRYPOINT ["/go/bin/terranotate"]
