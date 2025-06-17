FROM golang:1.24-bookworm as builder

WORKDIR /app
COPY . .
RUN go mod tidy && go build -ldflags="-s -w" -o whispyrBack .

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/whispyrBack .
CMD ["./whispyrBack"]
