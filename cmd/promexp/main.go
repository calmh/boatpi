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
	mo := flag.Float64("mo", 0, "Magnetic compass offset (degrees)")
	calfile := flag.String("calibration-file", "calibration.lsm9ds1", "Calibration file")
	flag.Parse()

	dev, err := sysfs.NewI2cDevice(*device)
	if err != nil {
		log.Fatalln("open I2C device:", err)
	}

	var update funcs

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
	alsm9ds1 := NewAvgLSM9DS1(time.Minute, 500*time.Millisecond, lsm9ds1)
	update = append(update, registerLSM9DS1(alsm9ds1))

	omini := omini.New(dev)
	update = append(update, registerOmini(omini))

	go func() {
		update.call()
		for range time.NewTicker(1 * time.Second).C {
			update.call()
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

type funcs []func()

func (fs funcs) call() {
	for _, fn := range fs {
		fn()
	}
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

	buckets := []float64{0}
	for i := 1; i < 10; i++ {
		buckets = append([]float64{float64(-i)}, buckets...)
		buckets = append(buckets, float64(i))
	}
	for i := 10; i < 20; i += 2 {
		buckets = append([]float64{float64(-i)}, buckets...)
		buckets = append(buckets, float64(i))
	}
	for i := 20; i < 50; i += 5 {
		buckets = append([]float64{float64(-i)}, buckets...)
		buckets = append(buckets, float64(i))
	}

	accelAH := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "sensors",
		Subsystem: "lsm9ds1",
		Name:      "accel_angle_degrees_histogram",
		Buckets:   buckets,
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
		xy, xz, yz := lsm9ds1.MedianAccelerationAngles()
		accelA.WithLabelValues("xy").Set(round(xy, 2))
		accelA.WithLabelValues("xz").Set(round(xz, 2))
		accelA.WithLabelValues("yz").Set(round(yz, 2))
		xy, xz, yz = lsm9ds1.AccelerationAngles()
		accelAH.WithLabelValues("xy").Observe(xy)
		accelAH.WithLabelValues("xz").Observe(xz)
		accelAH.WithLabelValues("yz").Observe(yz)
		xy, xz, yz = lsm9ds1.Deviation()
		devA.WithLabelValues("xy").Set(round(xy, 2))
		devA.WithLabelValues("xz").Set(round(xz, 2))
		devA.WithLabelValues("yz").Set(round(yz, 2))
		xy, xz, yz = lsm9ds1.Compass()
		compA.WithLabelValues("xy").Set(round(xy, 2))
		compA.WithLabelValues("xz").Set(round(xz, 2))
		compA.WithLabelValues("yz").Set(round(yz, 2))

		x = abs(x)
		y = abs(y)
		z = abs(z)
		h := 0.0
		switch {
		case x > y && x > z:
			// x is down
			h = yz
		case y > x && y > z:
			// y is down
			h = xz
		case z > x && z > y:
			// z is down
			h = xy
		}
		compA.WithLabelValues("horiz").Set(round(h, 2))

		x, y, z = lsm9ds1.MagneticField()
		compF.WithLabelValues("x").Set(float64(x))
		compF.WithLabelValues("y").Set(float64(y))
		compF.WithLabelValues("z").Set(float64(z))
	}
}

func abs(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
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
