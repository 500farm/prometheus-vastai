FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS build-stage

ARG TARGETOS
ARG TARGETARCH

WORKDIR /usr/local/go/src/build

COPY src/* go.mod go.sum ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags='-s -w' -o /usr/local/bin/vastai_exporter .

FROM gcr.io/distroless/static-debian13:latest

COPY --from=build-stage /usr/local/bin/vastai_exporter /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/vastai_exporter"]
EXPOSE 8622