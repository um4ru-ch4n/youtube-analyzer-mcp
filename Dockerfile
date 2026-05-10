# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache \
    ffmpeg \
    python3 \
    py3-pip \
    nodejs

RUN pip3 install --break-system-packages yt-dlp

COPY --from=builder /bin/server /bin/server
COPY config.yaml /app/config.yaml

WORKDIR /app

ENTRYPOINT ["/bin/server"]
