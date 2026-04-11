ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/server
COPY go.mod go.sum ./
RUN go mod download && go mod verify 
COPY . .
RUN go build -v -o /run-server ./cmd/server/main.go 


FROM debian:bookworm

COPY --from=builder /run-server /usr/local/bin/
RUN chmod +x /usr/local/bin/run-server
CMD ["/usr/local/bin/run-server", "-host", "0.0.0.0", "-port", "8080"]
