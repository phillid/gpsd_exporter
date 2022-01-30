FROM golang:1.17-bullseye as build
WORKDIR /go/src/gpsd_exporter
COPY . .
RUN go get -d -v
RUN go build -o /go/bin/gpsd_exporter

FROM scratch
COPY --from=build /go/bin/gpsd_exporter /gpsd_exporter
WORKDIR /
EXPOSE 9477
ENTRYPOINT ["/gpsd_exporter"]
