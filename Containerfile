FROM golang:1.17-bullseye as build
WORKDIR /go/src/gpsd-exporter
COPY . .
RUN go get -d -v
RUN go build -o /go/bin/gpsd-exporter

FROM scratch
COPY --from=build /go/bin/gpsd-exporter /gpsd-exporter
WORKDIR /
ENTRYPOINT ["/gpsd-exporter"]
