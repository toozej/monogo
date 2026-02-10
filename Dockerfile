# setup project and deps
FROM golang:1.25-bookworm AS init

WORKDIR /go/kmhd2spotify/

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

# Install coreutils for sleep and other utilities utilized in devcontainer
RUN apt-get update && apt-get install --no-install-recommends -y coreutils

RUN CGO_ENABLED=0 go build -ldflags="${LDFLAGS}"

# runtime image including CA certs and tzdata
FROM gcr.io/distroless/static-debian12:latest
# Copy our static executable.
COPY --from=build /go/kmhd2spotify/kmhd2spotify /go/bin/kmhd2spotify
# Expose port for publishing as web service
EXPOSE 8080
# Run the binary.
USER nonroot
ENTRYPOINT ["/go/bin/kmhd2spotify"]
