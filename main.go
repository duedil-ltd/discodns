package main

import (
	"log"
	"flag"
	"os"
	"os/signal"
)

var (
	logger = log.New(os.Stdout, "[zoodns] ", log.Ldate|log.Ltime)
)

func main() {

	var addr = flag.String("listen", "0.0.0.0", "Listen IP address")
	var port = flag.Int("port", 53, "Port to listen on")
	flag.Parse()

	server := &Server{
		addr: *addr,
		port: *port}

	server.Run()

	logger.Printf("Listening on %s:%d\n", *addr, *port)

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)

forever:
	for {
		select {
		case <-sig:
			logger.Printf("Bye bye :(\n")
			break forever
		}
	}
}
