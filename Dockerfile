FROM golang:alpine AS build-stage

WORKDIR /usr/local/go/src/build

COPY src/* go.mod go.sum ./
RUN go build -o /usr/local/bin/vastai_exporter .

FROM alpine

WORKDIR /usr/local/bin

COPY --from=build-stage /usr/local/bin/vastai_exporter ./

ENTRYPOINT ["/usr/local/bin/vastai_exporter"]
EXPOSE 8622
