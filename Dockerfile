# Build a static heramba binary and ship it on distroless (nonroot).
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/Armur-Ai/heramba/internal/version.Version=${VERSION}" \
      -o /heramba ./cmd/heramba

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /heramba /heramba
# Provide your own /.heramba.yml via a bind mount or a derived image.
EXPOSE 8080
ENTRYPOINT ["/heramba", "serve"]
