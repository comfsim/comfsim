ARG GO_VERSION
FROM golang:${GO_VERSION}-bookworm as builder
WORKDIR /work/builder
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /builder/server ./cmd/server/main.go

FROM debian:bookworm
COPY --from=builder /builder/server /out/
CMD ["/out/server", "-port", "8080", "-host", "0.0.0.0"]
