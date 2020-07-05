package main

import (
	"flag"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/kastelo-labs/sensehat"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gobot.io/x/gobot/sysfs"
)

func main() {
	device := flag.String("device", "/dev/i2c-1", "I2C device")
	promaddr := flag.String("prometheus", "", "Prometheus exporter address")
	po := flag.Float64("po", 0, "Pitch offset (degrees)")
	ro := flag.Float64("ro", 0, "Roll offset (degrees)")
	wo := flag.Float64("wo", 0, "Yaw offset (degrees)")
	flag.Parse()

	dev, err := sysfs.NewI2cDevice(*device)
	if err != nil {
		log.Fatalln("open I2C device:", err)
	}

	lps25h, err := sensehat.NewLPS25H(dev)
	if err != nil {
		log.Fatalln("init LPS25H:", err)
	}

	hts221, err := sensehat.NewHTS221(dev)
	if err != nil {
		log.Fatalln("init HTS221:", err)
	}

	lsm9ds1, err := sensehat.NewLSM9DS1(dev)
	if err != nil {
		log.Fatalln("init LSM9DS1:", err)
	}
	alsm9ds1 := sensehat.NewAvgLSM9DS1(time.Minute, 500*time.Millisecond, lsm9ds1, *po, *ro, *wo)

	if *promaddr != "" {
		servePrometheus(*promaddr, hts221, lps25h, alsm9ds1)
	}
}

func servePrometheus(addr string, hts221 *sensehat.HTS221, lps25h *sensehat.LPS25H, lsm9ds1 *sensehat.AvgLSM9DS1) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "hts221",
		Name:      "humidity_percent",
	}, func() float64 {
		hts221.Refresh(time.Second)
		return round(hts221.Humidity(), 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "hts221",
		Name:      "temperature_degC",
	}, func() float64 {
		hts221.Refresh(time.Second)
		return round(hts221.Temperature(), 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lps25h",
		Name:      "pressure_mb",
	}, func() float64 {
		lps25h.Refresh(time.Second)
		return round(lps25h.Pressure(), 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lps25h",
		Name:      "temperature_degC",
	}, func() float64 {
		lps25h.Refresh(time.Second)
		return round(lps25h.Temperature(), 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "median_deg",
		ConstLabels: prometheus.Labels{"direction": "pitch"},
	}, func() float64 {
		p, _, _ := lsm9ds1.Angles()
		return round(p, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "median_deg",
		ConstLabels: prometheus.Labels{"direction": "roll"},
	}, func() float64 {
		_, r, _ := lsm9ds1.Angles()
		return round(r, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "median_deg",
		ConstLabels: prometheus.Labels{"direction": "yaw"},
	}, func() float64 {
		_, _, w := lsm9ds1.Angles()
		return round(w, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "deviation_deg",
		ConstLabels: prometheus.Labels{"direction": "pitch"},
	}, func() float64 {
		p, _, _ := lsm9ds1.Deviation()
		return round(p, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "deviation_deg",
		ConstLabels: prometheus.Labels{"direction": "roll"},
	}, func() float64 {
		_, r, _ := lsm9ds1.Deviation()
		return round(r, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "deviation_deg",
		ConstLabels: prometheus.Labels{"direction": "yaw"},
	}, func() float64 {
		_, _, w := lsm9ds1.Deviation()
		return round(w, 2)
	})

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(addr, nil)
}

func round(x float64, prec int) float64 {
	pow := math.Pow10(prec)
	return math.Round(x*pow) / pow
}
