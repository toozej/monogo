# Keep this image in sync with the Go version declared in the root go.mod.
ARG GO_IMAGE=golang:1.26.7-trixie
FROM ${GO_IMAGE}

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# The Go image provides the compiler; these are the base utilities used by the
# repository's Make targets and tool-install scripts. Node, Python, and Docker
# are supplied by Dev Container Features in devcontainer.json.
RUN apt-get update \
    && apt-get install --no-install-recommends --yes \
        ca-certificates \
        curl \
        git \
        make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
