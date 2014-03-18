package main

import (
	"strconv"
	"github.com/miekg/dns"
	"time"
)

type Server struct {
	addr     string
	port     int
	rTimeout time.Duration
	wTimeout time.Duration
}

func (s *Server) Addr() string {
	return s.addr + ":" + strconv.Itoa(s.port)
}

func (s *Server) Run() {

	tcpHandler := dns.NewServeMux()
	tcpHandler.HandleFunc(".", LookupHandler)

	udpHandler := dns.NewServeMux()
	udpHandler.HandleFunc(".", LookupHandler)

	tcpServer := &dns.Server{Addr: s.Addr(),
		Net:          "tcp",
		Handler:      tcpHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second}

	udpServer := &dns.Server{Addr: s.Addr(),
		Net:          "udp",
		Handler:      udpHandler,
		UDPSize:      65535,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second}

	go s.start(udpServer)
	go s.start(tcpServer)
}

func (s *Server) start(ds *dns.Server) {

	err := ds.ListenAndServe()
	if err != nil {
		logger.Fatalf("Start %s listener on %s failed:%s", ds.Net, s.Addr(), err.Error())
	}
}
