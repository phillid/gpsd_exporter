package main

import (
	"log"
	"net/http"
	"flag"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/phillid/gpsd_exporter/gpsd"
)

type GPSdCollector struct {}

func (gc GPSdCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(gc, ch)
}

func (gc GPSdCollector) Collect(ch chan<- prometheus.Metric) {
	desc := prometheus.NewDesc(
		"gpsd_satellite_bloobloo_asdf",
		"Some description here",
		[]string{"host", "asdf"}, nil,
	)
	ch <- prometheus.MustNewConstMetric(
		desc,
		prometheus.GaugeValue,
		12345.6,
		"foo", "bar",
	)
}

var (
	metricsPath = flag.String("metrics-path", "/metrics", "HTTP path for the metrics endpoint")
	listenAddress = flag.String("listen-address", ":9478", "Address to listen on for HTTP requests")
	gpsdAddress = flag.String("gpsd-address", ":2947", "Address of gpsd server")
)

func main() {
	flag.Parse()

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		//prometheus.NewGoCollector(),
		GPSdCollector{},

	)

	session, err := gpsd.Dial(*gpsdAddress)
	if err != nil {
		log.Fatal(err)
	}
	session.SetVersionCallback(func(r gpsd.GPSDReportVersion) {
		log.Printf("Version: %#v\n", r)
	})
	session.SetSkyCallback(func(r gpsd.GPSDReportSky) {
		//log.Printf("Sky: %#v\n", r)
		total_used := 0
		for _, sat := range r.Satellites {
			log.Printf("* Satellite PRN %f Used %t GNSSID %f", sat.PRN, sat.Used, *sat.GNSSID)
			if sat.Used {
				total_used++
			}
		}
		log.Printf("In use: %d/%d\n", total_used, len(r.Satellites))
	})
	session.SetTPVCallback(func(r gpsd.GPSDReportTPV) {
		log.Printf("TPV: %f N %f E  %f m\n", *r.Latitude, *r.Longitude, *r.Altitude)
	})
	session.Watch()
	err = session.Run()
	if err != nil {
		log.Fatal(err)
	}
	session.Close()

	http.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title></title></head>
		<body>
		<h1>GPSd Exporter</h1>
		<p><a href="` + *metricsPath + `">Metrics</a></p>
		</body></html>`))
	})

	log.Print("gpsd-exporter listenting on " + *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
