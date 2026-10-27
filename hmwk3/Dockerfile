FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY internal ./internal
COPY server ./server

RUN CGO_ENABLED=0 go build -o server ./server/main.go

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/server /app/server

EXPOSE 8000

CMD ["/app/server"]
