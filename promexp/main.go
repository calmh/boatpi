package main

import (
	"encoding/binary"
	"flag"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/kastelo-labs/sensehat"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gobot.io/x/gobot/sysfs"
)

func main() {
	device := flag.String("device", "/dev/i2c-1", "I2C device")
	promaddr := flag.String("prometheus", ":9120", "Prometheus exporter address")
	ao := flag.Float64("ao", 0, "Accel A offset (degrees)")
	bo := flag.Float64("bo", 0, "Accel B offset (degrees)")
	co := flag.Float64("co", 0, "Accel C offset (degrees)")
	mo := flag.Float64("mo", 0, "Magnetic compass offset (degrees)")
	calfile := flag.String("calibration-file", "calibration.lsm9ds1", "Calibration file")
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

	cal := loadCalibration(*calfile)
	lsm9ds1, err := sensehat.NewLSM9DS1(dev, *mo, cal)
	if err != nil {
		log.Fatalln("init LSM9DS1:", err)
	}
	alsm9ds1 := NewAvgLSM9DS1(time.Minute, 500*time.Millisecond, lsm9ds1, *ao, *bo, *co)

	go func() {
		for range time.NewTicker(time.Minute).C {
			cur := lsm9ds1.Calibration()
			if cur != cal {
				saveCalibration(*calfile, cur)
				cal = cur
			}
		}
	}()

	servePrometheus(*promaddr, hts221, lps25h, alsm9ds1)
}

func servePrometheus(addr string, hts221 *sensehat.HTS221, lps25h *sensehat.LPS25H, lsm9ds1 *AvgLSM9DS1) {
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
		Name:        "accel_median_deg",
		ConstLabels: prometheus.Labels{"plane": "a"},
	}, func() float64 {
		a, _, _ := lsm9ds1.AccelAngles()
		return round(a, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "accel_median_deg",
		ConstLabels: prometheus.Labels{"plane": "b"},
	}, func() float64 {
		_, b, _ := lsm9ds1.AccelAngles()
		return round(b, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "accel_median_deg",
		ConstLabels: prometheus.Labels{"plane": "c"},
	}, func() float64 {
		_, _, c := lsm9ds1.AccelAngles()
		return round(c, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "accel_deviation_deg",
		ConstLabels: prometheus.Labels{"plane": "a"},
	}, func() float64 {
		a, _, _ := lsm9ds1.Deviation()
		return round(a, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "accel_deviation_deg",
		ConstLabels: prometheus.Labels{"plane": "b"},
	}, func() float64 {
		_, b, _ := lsm9ds1.Deviation()
		return round(b, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "accel_deviation_deg",
		ConstLabels: prometheus.Labels{"plane": "c"},
	}, func() float64 {
		_, _, c := lsm9ds1.Deviation()
		return round(c, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "compass_deg",
		ConstLabels: prometheus.Labels{"plane": "a"},
	}, func() float64 {
		a, _, _ := lsm9ds1.Compass()
		return round(a, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "compass_deg",
		ConstLabels: prometheus.Labels{"plane": "b"},
	}, func() float64 {
		_, b, _ := lsm9ds1.Compass()
		return round(b, 2)
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace:   "sensors",
		Subsystem:   "lsm9ds1",
		Name:        "compass_deg",
		ConstLabels: prometheus.Labels{"plane": "c"},
	}, func() float64 {
		_, _, c := lsm9ds1.Compass()
		return round(c, 2)
	})

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(addr, nil)
}

func round(x float64, prec int) float64 {
	pow := math.Pow10(prec)
	return math.Round(x*pow) / pow
}

func saveCalibration(file string, cal sensehat.Calibration) error {
	fd, err := os.Create(file)
	if err != nil {
		return err
	}
	if err := binary.Write(fd, binary.BigEndian, &cal); err != nil {
		fd.Close()
		return err
	}
	return fd.Close()
}

func loadCalibration(file string) sensehat.Calibration {
	fd, err := os.Open(file)
	if err != nil {
		return sensehat.Calibration{}
	}
	defer fd.Close()

	var cal sensehat.Calibration
	if err := binary.Read(fd, binary.BigEndian, &cal); err != nil {
		return sensehat.Calibration{}
	}

	return cal
}
