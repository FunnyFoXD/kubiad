FROM golang:1.26 AS builder
WORKDIR /workspace
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /manager ./cmd

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
