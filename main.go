package main

import (
    "github.com/coreos/go-etcd/etcd"
	"os/signal"
	"os"
	"strings"
	"log"
	"flag"
)

var (
	logger = log.New(os.Stdout, "[etcdns] ", log.Ldate|log.Ltime)
)

func main() {

	var addr = flag.String("listen", "0.0.0.0", "Listen IP address")
	var port = flag.Int("port", 53, "Port to listen on")
	var hosts = *flag.String("etcd", "0.0.0.0:4001", "List of etcd hosts (comma separated)")
	flag.Parse()

	// Connect to ZK (wait for a connection)
	etcd := etcd.NewClient(strings.Split(hosts, ","))

	if !etcd.SyncCluster() {
		logger.Fatalf("Failed to connect to etcd cluster")
	}

	// Start up the DNS resolver server
	server := &Server{
		addr: *addr,
		port: *port,
		etcd: *etcd}

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
