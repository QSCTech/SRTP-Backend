FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /srtp-backend ./cmd/server

FROM python:3.12-slim

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /srtp-backend /usr/local/bin/srtp-backend
COPY scripts/ ./scripts/
RUN pip install --no-cache-dir -r scripts/tyys_captcha_requirements.txt

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/srtp-backend"]
