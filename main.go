package main

import (
	"flag"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

type Game struct {
	debug      bool
	once       bool
	lastScreen *ebiten.Image

	windowWidth  int
	windowHeight int

	pixelUpdates chan PixelUpdate
}

type PixelUpdate struct {
	x     int32
	y     int32
	color color.RGBA
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.debug {
		defer log.Println("Screen updated")
	}

	if !g.once {
		g.once = true

		g.lastScreen = ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
	}

	if ebiten.IsKeyPressed(ebiten.KeyC) {
		screen.Fill(color.RGBA{0, 0, 0, 255})
	} else {
		screen.DrawImage(g.lastScreen, nil)
	}

	for {
		select {
		case update := <-g.pixelUpdates:
			screen.Set(int(update.x), int(update.y), update.color)
		default:
			g.lastScreen.DrawImage(screen, nil)
			return
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return outsideWidth, outsideHeight
}

func main() {
	// parse command line arguments for port number, width and height
	port := flag.Int("port", 1337, "port number")
	width := flag.Int("width", 800, "width")
	height := flag.Int("height", 600, "height")
	debug := flag.Bool("debug", false, "debug mode")
	flag.Parse()

	log.Println("Starting server on port", *port)
	log.Println("Serving", *width, "x", *height, "window")
	log.Println("Debug mode:", *debug)

	g := &Game{
		debug:        *debug,
		once:         false,
		windowWidth:  *width,
		windowHeight: *height,
		pixelUpdates: make(chan PixelUpdate, 210000),
	}

	// start server, listen on tcp port
	go func() {
		err := g.startServer(*port)
		if err != nil {
			log.Fatal(err)
		}
	}()

	ebiten.SetWindowSize(*width, *height)
	ebiten.SetWindowTitle("Hello, World!")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func (g *Game) startServer(port int) interface{} {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if g.debug {
				log.Println("Error accepting connection:", err)
			}
			continue
		}

		go g.handleConnection(conn)
	}

	return nil
}

func (g *Game) handleConnection(conn net.Conn) {
	defer conn.Close()

	// read data
	buf := make([]byte, 10240)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				if g.debug {
					log.Println("Error reading:", err)
				}
			}
			if g.debug {
				log.Println("Connection closed")
			}
			return
		}

		lastNewlineIndex := -1
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				g.handleLine(string(buf[lastNewlineIndex+1:i]), conn)
				lastNewlineIndex = i
			}
		}
		copy(buf, buf[lastNewlineIndex+1:])
	}
}

func (g *Game) handleLine(line string, conn net.Conn) {
	if g.debug {
		//log.Println("Received:", line)
		defer log.Println("Handled line")
	}

	if strings.HasPrefix(line, "SIZE") {
		// send window size
		_, err := conn.Write([]byte(fmt.Sprintf("SIZE %d %d\n", g.windowWidth, g.windowHeight)))
		if err != nil {
			return
		}
	} else if strings.HasPrefix(line, "PX") {
		fields := strings.Split(line, " ")
		if g.debug {
			//log.Println(fields)
		}
		if len(fields) == 3 {
			x, err := strconv.Atoi(fields[1])
			if err != nil {
				return
			}
			y, err := strconv.Atoi(fields[2])
			if err != nil {
				return
			}

			if x < 0 || x >= g.windowWidth || y < 0 || y >= g.windowHeight {
				return
			}

			// get colorAt from screen
			colorAt := g.lastScreen.At(x, y).(color.RGBA)
			// convert to hex string
			colorString := fmt.Sprintf("%02x%02x%02x", colorAt.R, colorAt.G, colorAt.B)

			_, err = conn.Write([]byte(fmt.Sprintf("PX %d %d %s\n", x, y, colorString)))
		} else if len(fields) == 4 {
			x, err := strconv.Atoi(fields[1])
			if err != nil {
				return
			}
			y, err := strconv.Atoi(fields[2])
			if err != nil {
				return
			}
			colorString := fields[3]

			if len(colorString) == 6 {
				r, err := strconv.ParseInt(colorString[0:2], 16, 0)
				if err != nil {
					return
				}
				gr, err := strconv.ParseInt(colorString[2:4], 16, 0)
				if err != nil {
					return
				}
				b, err := strconv.ParseInt(colorString[4:6], 16, 0)
				if err != nil {
					return
				}

				g.pixelUpdates <- PixelUpdate{
					x:     int32(x),
					y:     int32(y),
					color: color.RGBA{uint8(r), uint8(gr), uint8(b), 255},
				}
			} else if len(colorString) == 8 {
				r, err := strconv.ParseInt(colorString[0:2], 16, 0)
				if err != nil {
					return
				}
				gr, err := strconv.ParseInt(colorString[2:4], 16, 0)
				if err != nil {
					return
				}
				b, err := strconv.ParseInt(colorString[4:6], 16, 0)
				if err != nil {
					return
				}
				a, err := strconv.ParseInt(colorString[6:8], 16, 0)
				if err != nil {
					return
				}

				g.pixelUpdates <- PixelUpdate{
					x:     int32(x),
					y:     int32(y),
					color: color.RGBA{uint8(r), uint8(gr), uint8(b), uint8(a)},
				}
			} else if len(colorString) == 2 {
				gray, err := strconv.ParseInt(colorString, 16, 0)
				if err != nil {
					return
				}

				g.pixelUpdates <- PixelUpdate{
					x:     int32(x),
					y:     int32(y),
					color: color.RGBA{uint8(gray), uint8(gray), uint8(gray), 255},
				}
			}
		}
	} else if strings.HasPrefix(line, "HELP") {
		_, err := conn.Write([]byte("Welcome to Pixelflut!\n\nCommands:\n    HELP                -> get this information page\n    SIZE                -> get the size of the canvas\n    PX <x> <y>          -> get the color of pixel (x, y)\n    PX <x> <y> <COLOR>  -> set the color of pixel (x, y)\n    OFFSET <x> <y>      -> sets an pixel offset for all following commands\n\n    COLOR:\n        Grayscale: ww          (\"00\"       black .. \"ff\"       white)\n        RGB:       rrggbb      (\"000000\"   black .. \"ffffff\"   white)\n        RGBA:      rrggbbaa    (rgb with alpha)\n\nExample:\n    \"PX 420 69 ff\\n\"       -> set the color of pixel at (420, 69) to white\n    \"PX 420 69 00ffff\\n\"   -> set the color of pixel at (420, 69) to cyan\n    \"PX 420 69 ffff007f\\n\" -> blend the color of pixel at (420, 69) with yellow (alpha 127)\n"))
		if err != nil {
			return
		}
	}
}
