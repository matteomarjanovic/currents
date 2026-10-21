FROM --platform=$BUILDPLATFORM golang:1.26.10-bookworm AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o appview .

FROM debian:bookworm-slim

# Require the eMudhra distrust update, including for wget's HTTPS checks.
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && dpkg --compare-versions "$(dpkg-query -W -f='${Version}' ca-certificates)" ge '20250419~deb12u2' \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/appview .

EXPOSE 8080

ENTRYPOINT ["/app/appview"]
