# Build one static binary, ship it in a minimal runtime image.
# The match runs it with no network, a read-only root (writable /tmp), 1 CPU, 512 MB, uid 65534,
# and the map mounted at /arena. The bot never writes files.
FROM golang:1.26.1-bookworm@sha256:ab3d6955bbc813a0f3fdf220c1d817dd89c0b3f283777db8ece4a32fe7858edd AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY pilot ./pilot
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOFLAGS=-mod=mod \
    go build -trimpath -ldflags="-s -w" -o /bot . \
    && chmod 0555 /bot

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /bot /bot
USER 65534:65534
ENTRYPOINT ["/bot"]
