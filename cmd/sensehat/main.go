package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"log"
	"math"
	"os"
	"time"

	"github.com/kastelo-labs/sensehat"
	"gobot.io/x/gobot/sysfs"
)

func main() {
	device := flag.String("device", "/dev/i2c-1", "I2C device")
	interval := flag.Duration("interval", time.Second, "Interval between measurements")
	decimals := flag.Int("decimals", 2, "Rounding precision")
	buffer := flag.Bool("buffer", false, "Use output buffering")
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

	fields := make(map[string]interface{})
	out := io.Writer(os.Stdout)
	if *buffer {
		out = bufio.NewWriter(out)
	}
	enc := json.NewEncoder(out)

	for now := range time.NewTicker(*interval).C {
		fields["when"] = now

		if press, temp, err := lps25h.Data(); err != nil {
			log.Fatalln("lps25h:", err)
		} else {
			fields["lps25h_pressure_hpa"] = round(press, *decimals)
			fields["lps25h_temperature_c"] = round(temp, *decimals)
		}

		if hum, temp, err := hts221.Data(); err != nil {
			log.Fatalln("hts221:", err)
		} else {
			fields["hts221_humidity_rh"] = round(hum, *decimals)
			fields["hts221_temperature_c"] = round(temp, *decimals)
		}

		enc.Encode(fields)
	}
}

// round returns the half away from zero rounded value of x with prec precision.
//
// Special cases are:
// 	Round(±0) = +0
// 	Round(±Inf) = ±Inf
// 	Round(NaN) = NaN
func round(x float64, prec int) float64 {
	if x == 0 {
		// Make sure zero is returned
		// without the negative bit set.
		return 0
	}
	// Fast path for positive precision on integers.
	if prec >= 0 && x == math.Trunc(x) {
		return x
	}
	pow := math.Pow10(prec)
	intermed := x * pow
	if math.IsInf(intermed, 0) {
		return x
	}
	if x < 0 {
		x = math.Ceil(intermed - 0.5)
	} else {
		x = math.Floor(intermed + 0.5)
	}

	if x == 0 {
		return 0
	}

	return x / pow
}
