FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o server_bin ./server/main.go

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/server_bin /app/server_bin

EXPOSE 8000
CMD ["/app/server_bin"]