#!/usr/bin/env python

from prometheus_client.core import GaugeMetricFamily, REGISTRY
from prometheus_client import start_http_server
import logging
import gps
import time
import threading

report_cache_lock = threading.Lock()
report_cache = dict()
# Structure of report_cache:
## report_cache = {
##     "/dev/ttyS0": {
##         "tpv": {
##             "seen": 1643426090,
##             "report": {
##                 "lat": 45,
##                 "long": 23,
##                 ...
##             }
##         },
##         "sky": {
##             "seen": 1643426090,
##             "report": {
##                 ...
##                 "satellites": [
##                     { "used": True, "el": 86.5 ...},
##                     { "used": True, "el": 23.2 ...},
##                     ...
##                 ]
##             }
##         }
##     },
##     ...
## }

def is_expired(timestamp, max_age):
    return time.monotonic() > timestamp + max_age


class GpsdCollector(object):
    satellite_metric_configs = {
        'az': ('gpsd_satellite_azimuth_degrees', 'Satellite azimuth in degrees from true north'),
        'el': ('gpsd_satellite_elevation_degrees', 'Satellite elevation in degrees above the horizon'),
        'ss': ('gpsd_satellite_snr_dbhz', 'Satellite signal-to-noise ratio in decibel-hertz'),
        'used': ('gpsd_satellite_used', 'Whether the satellite is used to determine fix'),
        'gnssid': ('gpsd_satellite_gnssid', 'Satellite GNSS ID'),
        'svid': ('gpsd_satellite_svid', 'Satellite ID within its constellation'),
        'sigid': ('gpsd_satellite_sigid', 'Signal ID'),
        'freqid': ('gpsd_satellite_freqid', 'Frequency ID (GLONASS only)'),
        'health': ('gpsd_satellite_health', 'Satellite health'),
        # FIXME omitting: lots of optionals
    }
    fix_metric_configs = {
        'lat': ('gpsd_gps_fix_latitude_degrees', 'GPS fix latitude in degrees north of the equator'),
        'lon': ('gpsd_gps_fix_longitude_degrees', 'GPS fix longitude in degrees east of the prime meridian'),
        'alt': ('gpsd_gps_fix_altitude_meters', 'GPS fix altitude in meters above mean sea level (MSL)'),
        'mode': ('gpsd_gps_fix_mode', 'NMEA fix mode'),
        'status': ('gpsd_gps_fix_status', 'GPS fix status'),
        # FIXME omitting: ept epx spy epv track speed climb eps epc ecefx ecefy ecefz ecefvx ecefvy ecefvz ecefvAcc evefvAcc
    }
    def __init__(self, tpv_expiry=10, sky_expiry=60):
        self._tpv_expiry = tpv_expiry
        self._sky_expiry = sky_expiry


    def collect(self):
        with report_cache_lock:
            for device, reports in report_cache.items():
                # SKY metrics
                # FIXME bother with xdop, vdop etc?
                if 'sky' in reports:
                    # FIXME parameterise expiry
                    if is_expired(reports['sky']['seen'], self._tpv_expiry):
                        del reports['sky']
                    else:
                        report = reports['sky']['report']
                        for satellite_key, (metric_name, help_text) in self.satellite_metric_configs.items():
                            g = GaugeMetricFamily(metric_name, help_text, labels=['device', 'prn'])
                            for sat in report['satellites']:
                                if satellite_key in sat:
                                    g.add_metric([device, str(sat['PRN'])], sat[satellite_key])
                            yield g
                # TPV metrics
                if 'tpv' in reports:
                    # FIXME parameterise expiry
                    if is_expired(reports['tpv']['seen'], self._sky_expiry):
                        del reports['tpv']
                    else:
                        report = reports['tpv']['report']
                        for fix_key, (metric_name, help_text) in self.fix_metric_configs.items():
                            if fix_key in report:
                                g = GaugeMetricFamily(metric_name, help_text, labels=['device'])
                                g.add_metric([device], report[fix_key])
                                yield g

if __name__ == "__main__":
    REGISTRY.register(GpsdCollector())
    start_http_server(9477)
    logging.basicConfig(level=logging.DEBUG)
    session = gps.gps(mode=gps.WATCH_ENABLE)
    # FIXME reconnect on gpsd exit or socket errors?
    while True:
        report = session.next()
        device_extra = f' from {report["device"]}' if 'device' in report else ''
        logging.debug(f'{report["class"]}{device_extra}')
        if "device" in report:
            with report_cache_lock:
                report_cache_device = report_cache.setdefault(report['device'], dict())
                class_norm = report['class'].lower()
                report_cache_device_class = report_cache_device.setdefault(class_norm, dict())
                report_cache_device_class['report'] = report
                report_cache_device_class['seen'] = time.monotonic()
