FROM golang:1.26-alpine AS builder

WORKDIR /build

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN swag init -g server/main.go -o docs

RUN CGO_ENABLED=0 go build -o /build/server_bin ./server/main.go

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /build/server_bin /app/server_bin

EXPOSE 8000
CMD ["/app/server_bin"]