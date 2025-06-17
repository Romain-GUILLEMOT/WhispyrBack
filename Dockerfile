FROM golang:1.24.4-bookworm as builder

WORKDIR /app
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o whispyrBack .

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/whispyrBack .
CMD ["./whispyrBack"]
