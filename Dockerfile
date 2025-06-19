# syntax=docker/dockerfile:1
ARG BUILDER_IMAGE=golang:1.23
ARG DOCKER_IMAGE=ubuntu
ARG USERNAME=bot

FROM ${BUILDER_IMAGE} AS builder
WORKDIR /app
COPY main.go go.mod go.sum ./
COPY pkg ./pkg
ARG TARGETARCH
RUN --mount=type=cache,sharing=locked,target=/go/pkg/mod,id=go-pkg-mod-${TARGETARCH} \
    --mount=type=cache,sharing=locked,target=/root/.cache,id=root-cache-${TARGETARCH} \
    CGO_ENABLED=0 go build -o cli_discord_bot2

FROM ${DOCKER_IMAGE} AS final
ARG TARGETARCH USERNAME
RUN --mount=type=cache,sharing=locked,target=/var/cache/apt,id=var-cache-apt-${TARGETARCH} \
    --mount=type=cache,sharing=locked,target=/var/lib/apt,id=var-lib-apt-${TARGETARCH} \
    apt-get -U install --no-install-recommends -qy ca-certificates
COPY --from=builder /app/cli_discord_bot2 /usr/local/bin/
RUN useradd -m $USERNAME
USER $USERNAME
ENTRYPOINT ["/usr/local/bin/cli_discord_bot2"]
