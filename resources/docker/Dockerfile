FROM golang:1.21 AS build

ENV CGO_ENABLED=0
ENV GOOS=linux
RUN useradd -u 10001 wombat
RUN sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

WORKDIR /go/src/github.com/wombatwisdom/wombat/
# Update dependencies: On unchanged dependencies, cached layer will be reused
COPY go.* /go/src/github.com/wombatwisdom/wombat/
RUN go mod download

# Build
COPY . /go/src/github.com/wombatwisdom/wombat/
# Tag timetzdata required for busybox base image:
# https://github.com/benthosdev/benthos/issues/897
RUN task build TAGS="timetzdata"

# Pack
FROM busybox AS package

LABEL maintainer="Daan Gerits <daan@shono.io>"
LABEL org.opencontainers.image.source="https://github.com/wombatwisdom/wombat"

WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/src/github.com/wombatwisdom/wombat/target/bin/wombat .
COPY resources/docker/docker.yaml /wombat.yaml

USER wombat

EXPOSE 4195

ENTRYPOINT ["/wombat"]

CMD ["-c", "/wombat.yaml"]
