FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum (if it exists)
COPY go.mod ./
# RUN go mod download

COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o dxservice ./cmd/dxservice/main.go

# Final stage
FROM alpine:latest

WORKDIR /root/

# Copy the binary from the builder
COPY --from=builder /app/dxservice .
# Copy default config if it exists
# COPY config.json .

# Expose the telnet port
EXPOSE 7300

ENTRYPOINT ["./dxservice"]
