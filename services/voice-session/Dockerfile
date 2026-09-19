FROM golang:1.21-alpine AS builder
WORKDIR /app

# Resolve module dependencies before copying the source tree so checksum entries are available.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /voice-session ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /voice-session .
EXPOSE 8080
CMD ["/app/voice-session"]
