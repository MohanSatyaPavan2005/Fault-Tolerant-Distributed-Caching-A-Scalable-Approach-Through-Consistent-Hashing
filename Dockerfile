# Base Dockerfile for both master and auxiliary servers
FROM golang:1.21-alpine

# Set working directory
WORKDIR /app

# Install required packages for health checks and debugging
RUN apk add --no-cache curl tzdata ca-certificates

# Copy Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build arguments for server type (master or auxiliary)
ARG SERVER_TYPE=master

# Echo the server type for debugging
RUN echo "Building for SERVER_TYPE: ${SERVER_TYPE}"

# Build the application
RUN case "${SERVER_TYPE}" in \
      "master") \
        echo "Building master server" && \
        go build -o server ./cmd/master \
        ;; \
      *) \
        echo "Building auxiliary server" && \
        go build -o server ./cmd/auxiliary \
        ;; \
    esac

# Set environment variables (can be overridden at runtime)
ENV PORT=8000

# Expose the port
EXPOSE ${PORT}

# Command to run the application
CMD ["./server"]

