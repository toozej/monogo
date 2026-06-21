# setup project and deps
FROM golang:1.26-trixie AS init

WORKDIR /go/go-sort-out-gh-actions/

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
FROM gcr.io/distroless/static-debian13:nonroot
# Copy our static executable.
COPY --from=build /go/go-sort-out-gh-actions/go-sort-out-gh-actions /go/bin/go-sort-out-gh-actions
# Expose port for publishing as web service
# EXPOSE 8081
# Run the binary.
USER nonroot
ENTRYPOINT ["/go/bin/go-sort-out-gh-actions"]
