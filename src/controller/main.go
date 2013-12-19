package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/kid0m4n/go-rpi/i2c"
)

var (
	camWidth         = flag.Int("camw", 640, "width of the captured camera image")
	camHeight        = flag.Int("camh", 480, "height of the captured camera image")
	camFps           = flag.Int("fps", 1, "fps for camera")
	arduinoAddrStr   = flag.String("addr", "0x50", "arduino i2c address")
	fakeCar          = flag.Bool("fcr", false, "fake the car")
	fakeCam          = flag.Bool("fcm", false, "fake the camera")
	echoPinNumber    = flag.Int("epn", 10, "GPIO pin connected to the echo pad")
	triggerPinNumber = flag.Int("tpn", 9, "GPIO pin connected to the trigger pad")
)

func main() {
	log.Print("Hey! Starting up...")

	flag.Parse()

	var cam Camera = NullCamera
	if !*fakeCam {
		cam = NewCamera(*camWidth, *camHeight, *camFps)
	}
	defer cam.Close()
	cam.Run()

	arduinoAddr, err := strconv.ParseInt(*arduinoAddrStr, 0, 0)
	if err != nil {
		log.Fatalf("Could not parse %q for arduino i2c address", *arduinoAddrStr)
	}
	var car Car = NullCar
	if !*fakeCar {
		car = NewCar(i2c.Default, byte(arduinoAddr))
	}

	comp := NewCompass(i2c.Default)
	defer comp.Close()
	comp.Run()

	rf := NewRangeFinder(*echoPinNumber, *triggerPinNumber)

	ws := NewWebServer(car, cam, comp, rf)
	go pollforDistance(rf)

	ws.Run()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, os.Kill)
	<-quit

	log.Print("All done")

}

func pollforDistance(dist RangeFinder) {
	var car Car = NullCar
	if !*fakeCar {
		car = NewCar(i2c.Default, byte(0x50))
	}

	for {

		distance, err := dist.Distance()
		if err != nil {
			log.Panic(err)
		}
		fmt.Println(distance)
		time.Sleep(100 * time.Millisecond)
		if distance <= 25 {
			car.Speed(0)
		}

	}
}
