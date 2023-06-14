# Use a Golang base image
FROM golang:1.20.5-bullseye AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the source code into the container
COPY . .

# Build the Go application
RUN go build -o app

FROM debian:bullseye

# Set the working directory inside the container
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/app .

# Set the command to run the binary when the container starts
CMD ["./app"]
