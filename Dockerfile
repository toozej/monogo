# setup project and deps
FROM golang:1.26-trixie AS init

WORKDIR /go/podgrab/

COPY go.mod* go.sum* ./
RUN go mod download

COPY . ./

FROM init AS vet
RUN go vet ./...

# run tests
FROM init AS test
RUN go test -coverprofile c.out -v ./...

# build binary
FROM init AS build
ARG LDFLAGS

RUN CGO_ENABLED=0 go build -ldflags="${LDFLAGS}"

# runtime image including CA certs and tzdata
FROM gcr.io/distroless/static-debian13:latest
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
