# Build the binary
FROM docker.io/library/golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy dependencies
COPY vendor/ vendor/

# Copy the go source
COPY main.go main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X main.appVersion=0.1.0" -o prometheus-sample-app --installsuffix cgo main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/prometheus-sample-app .

USER 65532:65532

ENTRYPOINT ["/prometheus-sample-app"]
