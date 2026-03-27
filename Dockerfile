FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /srtp-backend ./cmd/server

FROM alpine:3.22

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /srtp-backend /usr/local/bin/srtp-backend

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/srtp-backend"]
