# Build stage
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go.mod and go.sum (if it exists)
COPY go.mod ./
# RUN go mod download

COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o spotmux ./cmd/spotmux/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies and create non-root user
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S spotmux \
    && adduser -S spotmux -G spotmux

WORKDIR /home/spotmux

# Copy the binary from the builder
COPY --from=builder /app/spotmux .
RUN chown spotmux:spotmux spotmux

# Use non-root user
USER spotmux

# Expose the telnet port
EXPOSE 7300

ENTRYPOINT ["./spotmux"]
