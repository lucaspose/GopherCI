# --- build ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

# --- runtime ---
# Pipeline steps run inside this container, so it ships with git, ssh and the
# Go toolchain to be able to build and test Go repositories out of the box.
FROM golang:1.26-alpine
RUN apk add --no-cache git openssh-client ca-certificates \
 && adduser -D -h /home/goci goci \
 && mkdir -p /app/artifacts && chown goci:goci /app/artifacts
WORKDIR /app
COPY --from=build /out/server ./server
COPY migrations ./migrations
USER goci
ENV HOME=/home/goci GOPATH=/home/goci/go GOCACHE=/home/goci/.cache/go-build
EXPOSE 8080
CMD ["./server"]
