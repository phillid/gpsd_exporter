package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/phillid/gpsd_exporter/gpsd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type GPSDCollector struct {
	mu sync.Mutex
	// FIXME split on device
	// FIXME expire when stale
	lastTPV        gpsd.GPSDReportTPV
	lastSky        gpsd.GPSDReportSky
	skyMetricDescs map[string]*prometheus.Desc

	// Per-satellite data
	SatelliteAzimuthDegrees    *prometheus.Desc
	SatelliteElevationDegrees  *prometheus.Desc
	SatelliteSignalToNoiseDBHZ *prometheus.Desc
	SatelliteUsed              *prometheus.Desc
	SatelliteGNSSID            *prometheus.Desc
	SatelliteSVID              *prometheus.Desc
	SatelliteSigID             *prometheus.Desc
	SatelliteFreqID            *prometheus.Desc
	SatelliteHealth            *prometheus.Desc

	// GPS fix data
	FixLatitudeDegrees  *prometheus.Desc
	FixLongitudeDegrees *prometheus.Desc
	FixAltitudeMeters   *prometheus.Desc
	FixMode             *prometheus.Desc
	FixStatus           *prometheus.Desc
}

func NewGPSDCollector() *GPSDCollector {
	satLabelNames := []string{"device", "prn"}
	fixLabelNames := []string{"device"}
	return &GPSDCollector{
		SatelliteAzimuthDegrees: prometheus.NewDesc(
			"gpsd_satellite_azimuth_degrees",
			"Satellite azimuth in degrees from true north",
			satLabelNames,
			nil,
		),
		SatelliteElevationDegrees: prometheus.NewDesc(
			"gpsd_satellite_elevation_degrees",
			"Satellite elevation in degrees above the horizon",
			satLabelNames,
			nil,
		),
		SatelliteSignalToNoiseDBHZ: prometheus.NewDesc(
			"gpsd_satellite_snr_dbhz",
			"Satellite signal-to-noise ratio in decibel-hertz",
			satLabelNames,
			nil,
		),
		SatelliteUsed: prometheus.NewDesc(
			"gpsd_satellite_used",
			"Whether the satellite is used to determine fix",
			satLabelNames,
			nil,
		),
		SatelliteGNSSID: prometheus.NewDesc(
			"gpsd_satellite_gnssid",
			"Satellite GNSS ID",
			satLabelNames,
			nil,
		),
		SatelliteSVID: prometheus.NewDesc(
			"gpsd_satellite_svid",
			"Satellite ID within its constellation",
			satLabelNames,
			nil,
		),
		SatelliteSigID: prometheus.NewDesc(
			"gpsd_satellite_sigid",
			"Signal ID",
			satLabelNames,
			nil,
		),
		SatelliteFreqID: prometheus.NewDesc(
			"gpsd_satellite_freqid",
			"Frequency ID (GLONASS only)",
			satLabelNames,
			nil,
		),
		SatelliteHealth: prometheus.NewDesc(
			"gpsd_satellite_health",
			"Satellite health",
			satLabelNames,
			nil,
		),
		FixLatitudeDegrees: prometheus.NewDesc(
			"gpsd_gps_fix_latitude_degrees",
			"GPS fix latitude in degrees north of the equator",
			fixLabelNames,
			nil,
		),
		FixLongitudeDegrees: prometheus.NewDesc(
			"gpsd_gps_fix_longitude_degrees",
			"GPS fix longitude in degrees east of the prime meridian",
			fixLabelNames,
			nil,
		),
		FixAltitudeMeters: prometheus.NewDesc(
			"gpsd_gps_fix_altitude_meters",
			"GPS fix altitude in meters above mean sea level (MSL)",
			fixLabelNames,
			nil,
		),
		FixMode: prometheus.NewDesc(
			"gpsd_gps_fix_mode",
			"NMEA fix mode",
			fixLabelNames,
			nil,
		),
		FixStatus: prometheus.NewDesc(
			"gpsd_gps_fix_status",
			"GPS fix status",
			fixLabelNames,
			nil,
		),
	}
}

func (gc GPSDCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(gc, ch)
}

func (gc GPSDCollector) Collect(ch chan<- prometheus.Metric) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	// FIXME we should be storing lastSky and lastTPV by device
	device := "/dev/ttyS0"

	tpv := gc.lastTPV
	optionalGauges := map[*prometheus.Desc]*float64{
		gc.FixLatitudeDegrees:  tpv.Latitude,
		gc.FixLongitudeDegrees: tpv.Longitude,
		gc.FixAltitudeMeters:   tpv.Altitude,
		gc.FixMode:             tpv.Mode,
		gc.FixStatus:           tpv.Status,
	}

	for desc, valuePointer := range optionalGauges {
		// Nil indicates an optional field that was missing on the wire
		if valuePointer != nil {
			ch <- prometheus.MustNewConstMetric(
				desc,
				prometheus.GaugeValue,
				*valuePointer,
				device,
			)
		}
	}

	for _, sat := range gc.lastSky.Satellites {
		labels := []string{device, fmt.Sprint(sat.PRN)}

		// Handle oddball metrics first
		usedFloat := float64(0)
		if sat.Used {
			usedFloat = 1
		}
		ch <- prometheus.MustNewConstMetric(
			gc.SatelliteUsed,
			prometheus.GaugeValue,
			usedFloat,
			labels...,
		)

		// Handle optional float64 gauges in bulk
		optionalGauges := map[*prometheus.Desc]*float64{
			gc.SatelliteAzimuthDegrees:    sat.Azimuth,
			gc.SatelliteElevationDegrees:  sat.Elevation,
			gc.SatelliteSignalToNoiseDBHZ: sat.SNR,
			gc.SatelliteGNSSID:            sat.GNSSID,
			gc.SatelliteSVID:              sat.SVID,
			gc.SatelliteSigID:             sat.SigID,
			gc.SatelliteFreqID:            sat.FreqID,
			gc.SatelliteHealth:            sat.Health,
		}
		for desc, valuePointer := range optionalGauges {
			// Nil indicates an optional field that was missing on the wire
			if valuePointer != nil {
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					*valuePointer,
					labels...,
				)
			}
		}
	}
}

func (gc *GPSDCollector) SetLatestTPV(r gpsd.GPSDReportTPV) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.lastTPV = r
}

func (gc *GPSDCollector) SetLatestSky(r gpsd.GPSDReportSky) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.lastSky = r
}

var (
	metricsPath   = flag.String("metrics-path", "/metrics", "HTTP path for the metrics endpoint")
	listenAddress = flag.String("listen-address", ":9477", "Address to listen on for HTTP requests")
	gpsdAddress   = flag.String("gpsd-address", ":2947", "Address of gpsd server")
)

func main() {
	flag.Parse()

	reg := prometheus.NewPedanticRegistry()
	gpsdCollector := NewGPSDCollector()
	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		//prometheus.NewGoCollector(),
		gpsdCollector,
	)

	// FIXME propagate gpsd session exit to http thread
	go func() {
		session, err := gpsd.Dial(*gpsdAddress)
		if err != nil {
			log.Fatal(err)
		}
		log.Print("gpsd-exporter connected to " + *gpsdAddress)

		session.SetVersionCallback(func(r gpsd.GPSDReportVersion) {
			log.Printf("Version: %#v\n", r)
		})
		session.SetSkyCallback(func(r gpsd.GPSDReportSky) {
			gpsdCollector.SetLatestSky(r)
		})
		session.SetTPVCallback(func(r gpsd.GPSDReportTPV) {
			gpsdCollector.SetLatestTPV(r)
		})

		// FIXME users of gpsd don't care about Watch, only Run
		session.Watch()
		err = session.Run()
		if err != nil {
			log.Fatal(err)
		}
		// FIXME think about this
		session.Close()
	}()

	http.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title></title></head>
		<body>
		<h1>GPSD Exporter</h1>
		<p><a href="` + *metricsPath + `">Metrics</a></p>
		</body></html>`))
	})

	log.Print("gpsd-exporter listenting on " + *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
