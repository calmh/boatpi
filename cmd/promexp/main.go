package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/calmh/boatpi/omini"
	"github.com/calmh/boatpi/sensehat"
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

	var update []func()

	lps25h, err := sensehat.NewLPS25H(dev)
	if err != nil {
		log.Fatalln("init LPS25H:", err)
	}
	update = append(update, registerLPS25H(lps25h))

	hts221, err := sensehat.NewHTS221(dev)
	if err != nil {
		log.Fatalln("init HTS221:", err)
	}
	update = append(update, registerHTS221(hts221))

	cal := loadCalibration(*calfile)
	lsm9ds1, err := sensehat.NewLSM9DS1(dev, *mo, cal)
	if err != nil {
		log.Fatalln("init LSM9DS1:", err)
	}
	alsm9ds1 := NewAvgLSM9DS1(time.Minute, 500*time.Millisecond, lsm9ds1, *ao, *bo, *co)
	update = append(update, registerLSM9DS1(alsm9ds1))

	omini := omini.New(dev)
	update = append(update, registerOmini(omini))

	go func() {
		for range time.NewTicker(15 * time.Second).C {
			for _, fn := range update {
				fn()
			}
		}
	}()

	go func() {
		for range time.NewTicker(time.Minute).C {
			cur := lsm9ds1.Calibration()
			if cur != cal {
				saveCalibration(*calfile, cur)
				cal = cur
			}
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(*promaddr, nil)
}

func registerHTS221(hts221 *sensehat.HTS221) func() {
	hum := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "hts221",
		Name:      "humidity_percent",
	})
	temp := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "hts221",
		Name:      "temperature_celsius",
	})

	return func() {
		if err := hts221.Refresh(time.Second); err != nil {
			log.Println("HTS221:", err)
			hum.Set(0)
			temp.Set(0)
			return
		}

		hum.Set(round(hts221.Humidity(), 2))
		temp.Set(round(hts221.Temperature(), 2))
	}
}

func registerLPS25H(lps25h *sensehat.LPS25H) func() {
	press := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lps25h",
		Name:      "pressure_mb",
	})

	temp := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lps25h",
		Name:      "temperature_celsius",
	})

	return func() {
		if err := lps25h.Refresh(time.Second); err != nil {
			log.Println("LPS25H:", err)
			press.Set(0)
			temp.Set(0)
			return
		}

		press.Set(round(lps25h.Pressure(), 2))
		temp.Set(round(lps25h.Temperature(), 2))
	}
}

func registerLSM9DS1(lsm9ds1 *AvgLSM9DS1) func() {
	accel := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lsm9ds1",
		Name:      "accel_field",
	}, []string{"direction"})

	accelA := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lsm9ds1",
		Name:      "accel_angle_degrees",
	}, []string{"plane"})

	devA := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lsm9ds1",
		Name:      "accel_deviation_degrees",
	}, []string{"plane"})

	compA := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lsm9ds1",
		Name:      "compass_degrees",
	}, []string{"plane"})

	compF := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "lsm9ds1",
		Name:      "magnetic_field",
	}, []string{"direction"})

	return func() {
		x, y, z := lsm9ds1.Acceleration()
		accel.WithLabelValues("x").Set(float64(x))
		accel.WithLabelValues("y").Set(float64(y))
		accel.WithLabelValues("z").Set(float64(z))
		a, b, c := lsm9ds1.AccelAngles()
		accelA.WithLabelValues("a").Set(round(a, 2))
		accelA.WithLabelValues("b").Set(round(b, 2))
		accelA.WithLabelValues("c").Set(round(c, 2))
		a, b, c = lsm9ds1.Deviation()
		devA.WithLabelValues("a").Set(round(a, 2))
		devA.WithLabelValues("b").Set(round(b, 2))
		devA.WithLabelValues("c").Set(round(c, 2))
		a, b, c = lsm9ds1.Compass()
		compA.WithLabelValues("a").Set(round(a, 2))
		compA.WithLabelValues("b").Set(round(b, 2))
		compA.WithLabelValues("c").Set(round(c, 2))

		x &^= 1 << 14
		y &^= 1 << 14
		z &^= 1 << 14
		sc := 0.0
		switch {
		case x > y && x > z:
			sc = b
		case y > x && y > z:
			sc = c
		case z > x && z > y:
			sc = a
		}
		compA.WithLabelValues("horiz").Set(round(sc, 2))

		x, y, z = lsm9ds1.MagneticField()
		compF.WithLabelValues("x").Set(float64(x))
		compF.WithLabelValues("y").Set(float64(y))
		compF.WithLabelValues("z").Set(float64(z))
	}
}

func registerOmini(omini *omini.Omini) func() {
	vv := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sensors",
		Subsystem: "omini",
		Name:      "voltage",
	}, []string{"channel"})

	return func() {
		a, b, c, err := omini.Voltages()
		if err != nil {
			log.Println("Omini:", err)
			vv.WithLabelValues("a").Set(0)
			vv.WithLabelValues("b").Set(0)
			vv.WithLabelValues("c").Set(0)
			return
		}

		vv.WithLabelValues("a").Set(a)
		vv.WithLabelValues("b").Set(b)
		vv.WithLabelValues("c").Set(c)
	}
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
	enc := json.NewEncoder(fd)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&cal); err != nil {
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
	dec := json.NewDecoder(fd)
	if err := dec.Decode(&cal); err != nil {
		return sensehat.Calibration{}
	}

	return cal
}
