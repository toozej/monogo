# setup project and deps
FROM golang:1.27.1-trixie AS init

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

WORKDIR /workspace/

COPY go.mod* go.sum* ./
RUN go mod download

# Copy only the selected app and shared packages so unrelated apps stay out of the build context
COPY apps/notes2ssg ./apps/notes2ssg
COPY pkg ./pkg

FROM init AS vet
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go vet ./apps/notes2ssg/... ./pkg/...

# run tests
FROM init AS test
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN go test -coverprofile c.out -v ./apps/notes2ssg/... ./pkg/... && \
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
    CGO_ENABLED=0 go build -o /out/notes2ssg -ldflags="${LDFLAGS}" ./apps/notes2ssg

# runtime image
FROM scratch
# Copy our executable.
COPY --from=build /out/notes2ssg /go/bin/notes2ssg
# Include the project license in the runtime image.
COPY LICENSE /usr/share/doc/notes2ssg/copyright
# Expose port for publishing as web service
# EXPOSE 8081
# Verify that the application binary can start without requiring a shell or
# additional utilities in the minimal runtime image.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/go/bin/notes2ssg", "version"]
# Run the binary.
USER 65532:65532
ENTRYPOINT ["/go/bin/notes2ssg"]
