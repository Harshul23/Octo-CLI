# Stage 1: Build the binary
FROM golang:1.25-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Octo binary (statically linked for portability)
RUN CGO_ENABLED=0 GOOS=linux go build -o octo ./cmd

# Stage 2: Final lightweight image
FROM alpine:3.19

# Install basic system tools often needed for local dev (like git/curl)
RUN apk --no-cache add ca-certificates git curl

WORKDIR /root/

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/octo .

# Ensure the binary is executable
RUN chmod +x ./octo

# Set the binary as the entrypoint
ENTRYPOINT ["./octo"]