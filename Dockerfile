ARG VERSION="dev"

FROM golang:1.25.0 AS build
# allow this step access to build arg
ARG VERSION
# Set the working directory
WORKDIR /build

RUN go env -w GOMODCACHE=/root/.cache/go-build

# Install dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

COPY . ./
# Build the server
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 make go-build

# Make a stage to run the app
FROM gcr.io/distroless/base-debian12
# Set the working directory
WORKDIR /
# Copy the binary from the build stage
COPY --from=build /build/bin/myshoes-mcp-server .
# Command to run the server
CMD ["/myshoes-mcp-server", "stdio"]