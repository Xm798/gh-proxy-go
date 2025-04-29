# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o gh-proxy-go

FROM --platform=$TARGETPLATFORM alpine:latest

WORKDIR /app

COPY --from=builder /app/gh-proxy-go .
COPY --from=builder /app/config.json .
COPY --from=builder /app/public ./public

EXPOSE 8080

CMD ["./gh-proxy-go"] 