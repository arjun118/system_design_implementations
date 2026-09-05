# build stage
FROM golang:1.26.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY cmd/ ./cmd
COPY internal/ ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api ./cmd/api

# run stage
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/api .

CMD ["./api"]
