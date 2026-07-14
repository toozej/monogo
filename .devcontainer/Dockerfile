# setup project and deps
ARG GO_IMAGE=golang:1.26-trixie
FROM ${GO_IMAGE} AS init

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG APP=monogo
ARG APP_CONFIG=
ENV APP=${APP}
ENV APP_CONFIG=${APP_CONFIG}

WORKDIR /workspace/

COPY go.mod* go.sum* ./
RUN go mod download

ARG TASK_VERSION=v3.52.0
RUN GOBIN=/usr/local/bin go install github.com/go-task/task/v3/cmd/task@${TASK_VERSION}

COPY . ./

FROM init AS vet
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN APP_CONFIG="${APP_CONFIG:-apps/${APP}/app.yaml}" && \
    test -f "${APP_CONFIG}" && \
    app_value() { awk -F': *' -v key="$1" '$1 == key {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}"; } && \
    APP_PATH="$(app_value path)" && \
    APP_PATH="${APP_PATH:-apps/${APP}}" && \
    go vet "./${APP_PATH}/..." ./pkg/...

# run tests
FROM init AS test
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN APP_CONFIG="${APP_CONFIG:-apps/${APP}/app.yaml}" && \
    test -f "${APP_CONFIG}" && \
    app_value() { awk -F': *' -v key="$1" '$1 == key {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}"; } && \
    APP_PATH="$(app_value path)" && \
    APP_PATH="${APP_PATH:-apps/${APP}}" && \
    go test -coverprofile c.out -v "./${APP_PATH}/..." ./pkg/... && \
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

RUN APP_CONFIG="${APP_CONFIG:-apps/${APP}/app.yaml}" && \
    test -f "${APP_CONFIG}" && \
    app_value() { awk -F': *' -v key="$1" '$1 == key {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}"; } && \
    APP_BINARY="$(app_value binary)" && \
    APP_BINARY="${APP_BINARY:-${APP}}" && \
    APP_MAIN_PATH="$(app_value mainPath)" && \
    APP_MAIN_PATH="${APP_MAIN_PATH:-apps/${APP}}" && \
    case "$(app_value cgoEnabled)" in true|1) CGO_ENABLED=1;; *) CGO_ENABLED=0;; esac && \
    MODULE="$(awk '/^module / {print $2; exit}' go.mod)" && \
    LDFLAGS="-s -w -X ${MODULE}/pkg/version.Version=${VERSION} -X ${MODULE}/pkg/version.Commit=${COMMIT} -X ${MODULE}/pkg/version.Branch=${BRANCH} -X ${MODULE}/pkg/version.BuiltAt=${BUILT_AT} -X ${MODULE}/pkg/version.Builder=${BUILDER}" && \
    CGO_ENABLED="${CGO_ENABLED}" go build -o "/out/${APP_BINARY}" -ldflags="${LDFLAGS}" "./${APP_MAIN_PATH}"

# Install coreutils for sleep and other utilities utilized in devcontainer
RUN apt-get update && apt-get install --no-install-recommends -y coreutils && rm -rf /var/lib/apt/lists/*
