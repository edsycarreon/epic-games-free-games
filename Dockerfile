FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/epic-games-api

# Use a distroless image for a smaller, more secure final image
FROM gcr.io/distroless/static-debian12

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/epic-games-api /app/epic-games-api
# Copy the environment file
COPY .env ./

# Run as non-privileged user
USER nonroot:nonroot

# Command to run
CMD ["/app/epic-games-api"] 