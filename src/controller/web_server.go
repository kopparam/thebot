package ain

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/codegangsta/martini"
	"github.com/gorilla/websocket"
)

type WebServer struct {
	m    *martini.ClassicMartini
	car  Car
	cam  Camera
	comp Compass
	rf   RangeFinder
}

func NewWebServer(car Car, cam Camera, comp Compass, rf RangeFinder) *WebServer {
	var ws WebServer

	ws.m = martini.Classic()
	ws.car = car
	ws.cam = cam
	ws.comp = comp
	ws.rf = rf

	ws.registerHandlers()

	return &ws
}

func (ws *WebServer) registerHandlers() {
	ws.m.Get("/ws", ws.wsHandler)
	ws.m.Post("/speed/:speed/angle/:angle", ws.setSpeedAndAngle)
	ws.m.Get("/orientation", ws.orientation)
	ws.m.Get("/distance", ws.distance)
	ws.m.Get("/snapshot", ws.snapshot)
	ws.m.Post("/reset", ws.reset)
	ws.m.Post("/setswing/:dest", ws.setSwing)
	ws.m.Get("/ws2", ws.ws2Handler)
}

func (ws *WebServer) Run() {
	log.Print("Starting web server")

	go ws.m.Run()
}

func (ws *WebServer) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r, nil, 1024*1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", http.StatusBadRequest)
		return
	} else if err != nil {
		log.Print(err)
		return
	}

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType == websocket.TextMessage {
			msg := string(p)
			parts := strings.Split(msg, ",")
			speedStr, angleStr := parts[0], parts[1]

			_, err = ws.setOrientation(speedStr, angleStr)
			if err != nil {
				log.Print(err)
			}
		}
	}
}

func (ws *WebServer) ws2Handler(w http.ResponseWriter, r *http.Request) {

	conn, err := websocket.Upgrade(w, r, nil, 1024*1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", http.StatusBadRequest)
		return
	} else if err != nil {
		log.Print(err)
		return
	}

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType == websocket.TextMessage {
			msg := string(p)
			ws.point(msg)

		}
	}
}

func (ws *WebServer) setSpeedAndAngle(w http.ResponseWriter, params martini.Params) {
	code, err := ws.setOrientation(params["speed"], params["angle"])

	if err != nil {
		http.Error(w, err.Error(), code)
	}
}

func (ws *WebServer) orientation(w http.ResponseWriter) string {
	heading, err := ws.comp.Heading()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return fmt.Sprintf("%v", heading)
}

func (ws *WebServer) distance(w http.ResponseWriter) string {
	distance, err := ws.rf.Distance()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return fmt.Sprintf("%v", distance)
}

func (ws *WebServer) snapshot(w http.ResponseWriter) {
	log.Print("Sending current snapshot")

	image := ws.cam.CurrentImage()
	w.Write(image)
}

func (ws *WebServer) reset(w http.ResponseWriter) {
	log.Print("Resetting...")
	if err := ws.car.Reset(); err != nil {
		http.Error(w, "could not reset", http.StatusInternalServerError)
	}
}

func (ws *WebServer) setOrientation(speedStr, angleStr string) (code int, err error) {
	speed, err := strconv.Atoi(speedStr)
	if err != nil {
		return http.StatusBadRequest, errors.New("speed not valid")
	}
	angle, err := strconv.Atoi(angleStr)
	if err != nil {
		return http.StatusBadRequest, errors.New("angle not valid")
	}
	log.Printf("Received orientation %v, %v", angle, speed)
	if err = ws.car.Speed(speed); err != nil {
		return http.StatusInternalServerError, err
	}
	if err = ws.car.Turn(angle); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
func (ws *WebServer) setSwing(w http.ResponseWriter, params martini.Params) {
	ws.point(params["dest"])

}

func (ws *WebServer) point(destStr string) {

	dest, _ := strconv.ParseFloat(destStr, 64)
	var dir int
	start, _ := ws.comp.Heading()
    flag_speed := true

	// Determine left/right
	swing := start - dest
    if swing <= -180 || (swing >= 0 && swing <= 180) {
		dir = -1
	} else if (swing > -180 && swing < 0) || swing > 180 {
		dir = 1
	}

	ws.car.Speed(0)
	ws.car.Turn(90)

	// Initial momentum
	ws.car.Speed(130)
	time.Sleep(1 * time.Second)

	// Start turning
	for i := 90; ; {
		// Exit if BOT reached destination
		head, _ := ws.comp.Heading()
		if head < dest+5 && head > dest-5 {
			ws.car.Speed(0)
			ws.car.Turn(90)
			break
		}
		// Turn slowly
		if i > 75 && i < 105 {
			i = i + dir
			ws.car.Turn(i)
			time.Sleep(10)
		}
		// Speed up to componsate for momentum loss during turn
		if (i < 83 || i > 97) && flag_speed {
			ws.car.Speed(150)
            flag_speed = false
		}
	}
}
