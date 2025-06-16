FROM golang:1.24 as builder

WORKDIR /app
COPY . .
RUN go mod tidy && go build -o whispyrBack .

FROM debian:bullseye-slim
WORKDIR /app
COPY --from=builder /app/whispyrBack .
CMD ["./whispyrBack"]
