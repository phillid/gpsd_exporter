package gpsd

import (
	"net"
	"encoding/json"
	"bufio"
	"log"
)

type GPSD struct {
	conn net.Conn
	reader *bufio.Reader

	versionCallback func(GPSDReportVersion)
	tpvCallback     func(GPSDReportTPV)
	skyCallback     func(GPSDReportSky)
}

// Data types for GPSD protocol "Reports"

// Generic type, enough to look inside and see the class for full decoding
type GPSDReportGeneric struct {
	Class string `json:"class"`
}

// VERSION: "The daemon ships a VERSION response to each client when the client first connects to it."
type GPSDReportVersion struct {
	Release     string  `json:"release"`
	Rev         string  `json:"rev"`
	Proto_major float64 `json:"proto_major"`
	Proto_minor float64 `json:"proto_minor"`
	Remote      *string  `json:"remote"`
}

// SKY: "sky view of the GPS satellite positions"
type GPSDReportSky struct {
	Device *string  `json:"device"`
	// fields not used have been omitted
	Satellites []GPSDSatellite `json:"satellites"`
}

// TPV: "time-position-velocity report"
type GPSDReportTPV struct {
	Device    *string  `json:"device"`
	Mode      float64 `json:"mode"`
	Status    *float64 `json:"status"`
	Latitude  *float64 `json:"lat"`
	Longitude *float64 `json:"lon"`
	Altitude  *float64 `json:"alt"`
}

// data types found inside Reports
type GPSDSatellite struct {
	PRN       float64 `json:"PRN"`
	Azimuth   *float64 `json:"az"`
	Elevation *float64 `json:"el"`
	SNR       *float64 `json:"ss"`
	GNSSID    *float64 `json:"gnssid"`
	SVID      *float64 `json:"svid"`
	SigID     *float64 `json:"sigid"`
	FreqID    *float64 `json:"freqid"`
	Health    *float64 `json:"health"`
	Used      bool    `json:"used"`
}

func Dial(address string) (gpsd GPSD, err error) {
	conn, err := net.Dial("tcp", address)
	gpsd = GPSD{
		conn: conn,
		reader: bufio.NewReader(conn),
	}
	return
}

func (gpsd GPSD) processNext() (err error) {
	var rawReport json.RawMessage
	rawReport, err = gpsd.reader.ReadBytes('\n')
	if err == nil {
		var genericReport GPSDReportGeneric
		err = json.Unmarshal(rawReport, &genericReport)
		if err == nil {
			switch class := genericReport.Class ; class {
			case "VERSION":
				if callback := gpsd.versionCallback; callback != nil {
					var versionReport GPSDReportVersion
					json.Unmarshal(rawReport, &versionReport)
					callback(versionReport)
				}
			case "SKY":
				if callback := gpsd.skyCallback; callback != nil {
					var skyReport GPSDReportSky
					json.Unmarshal(rawReport, &skyReport)
					callback(skyReport)
				}
			case "TPV":
				if callback := gpsd.tpvCallback; callback != nil {
					var tpvReport GPSDReportTPV
					json.Unmarshal(rawReport, &tpvReport)
					callback(tpvReport)
				}
			default:
				log.Print("ignoring unimplemented report class " + class)
			}
		}
	}
	return
}

func (gpsd GPSD) Run() (err error) {
	for err == nil {
		err = gpsd.processNext()
	}
	return
}

func (gpsd *GPSD) SetVersionCallback(callback func(GPSDReportVersion)) {
	gpsd.versionCallback = callback
}

func (gpsd *GPSD) SetSkyCallback(callback func(GPSDReportSky)) {
	gpsd.skyCallback = callback
}

func (gpsd *GPSD) SetTPVCallback(callback func(GPSDReportTPV)) {
	gpsd.tpvCallback = callback
}

func (gpsd GPSD) Watch() (err error) {
	// hardcode since GPSD commands aren't plain JSON, and this literal will never change™
	_, err = gpsd.conn.Write([]byte(`?WATCH={"enable": true, "json": true}`))
	return
}

func (gpsd GPSD) Close() {
	gpsd.conn.Close()
}
