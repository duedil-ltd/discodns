package main

import (
	"github.com/coreos/go-etcd/etcd"
	"github.com/miekg/dns"
	"github.com/rcrowley/go-metrics"
	"strconv"
	"time"
)

type Server struct {
	addr          string
	port          int
	etcd          *etcd.Client
	rTimeout      time.Duration
	wTimeout      time.Duration
	defaultTtl    uint32
	queryFilterer *QueryFilterer
}

type Handler struct {
	resolver      *Resolver
	queryFilterer *QueryFilterer

	// Metrics
	requestCounter metrics.Counter
	acceptCounter  metrics.Counter
	rejectCounter  metrics.Counter
	responseTimer  metrics.Timer
}

func (h *Handler) Handle(response dns.ResponseWriter, req *dns.Msg) {
	h.requestCounter.Inc(1)
	h.responseTimer.Time(func() {
		debugMsg("Handling incoming query for domain " + req.Question[0].Name)

		// Lookup the dns record for the request
		// This method will add any answers to the message
		var msg *dns.Msg
		if h.queryFilterer.ShouldAcceptQuery(req) != true {
			debugMsg("Query not accepted")

			h.rejectCounter.Inc(1)

			msg = new(dns.Msg)
			msg.SetReply(req)
			msg.SetRcode(req, dns.RcodeNameError)
			msg.Authoritative = true
			msg.RecursionAvailable = false

			// Add a useful TXT record
			header := dns.RR_Header{Name: req.Question[0].Name,
				Class:  dns.ClassINET,
				Rrtype: dns.TypeTXT}
			msg.Ns = []dns.RR{&dns.TXT{header, []string{"Rejected query based on matched filters"}}}
		} else {
			h.acceptCounter.Inc(1)
			msg = h.resolver.Lookup(req)
		}

		if msg != nil {
			err := response.WriteMsg(msg)
			if err != nil {
				debugMsg("Error writing message: ", err)
			}
		}

		debugMsg("Sent response to ", response.RemoteAddr())
	})
}

func (s *Server) Addr() string {
	return s.addr + ":" + strconv.Itoa(s.port)
}

func (s *Server) Run() {

	tcpResponseTimer := metrics.NewTimer()
	metrics.Register("request.handler.tcp.response_time", tcpResponseTimer)
	tcpRequestCounter := metrics.NewCounter()
	metrics.Register("request.handler.tcp.requests", tcpRequestCounter)
	tcpAcceptCounter := metrics.NewCounter()
	metrics.Register("request.handler.tcp.filter_accepts", tcpAcceptCounter)
	tcpRejectCounter := metrics.NewCounter()
	metrics.Register("request.handler.tcp.filter_rejects", tcpRejectCounter)

	udpResponseTimer := metrics.NewTimer()
	metrics.Register("request.handler.udp.response_time", udpResponseTimer)
	udpRequestCounter := metrics.NewCounter()
	metrics.Register("request.handler.udp.requests", udpRequestCounter)
	udpAcceptCounter := metrics.NewCounter()
	metrics.Register("request.handler.udp.filter_accepts", udpAcceptCounter)
	udpRejectCounter := metrics.NewCounter()
	metrics.Register("request.handler.udp.filter_rejects", udpRejectCounter)

	resolver := Resolver{etcd: s.etcd, defaultTtl: s.defaultTtl}
	tcpDNShandler := &Handler{
		resolver:       &resolver,
		requestCounter: tcpRequestCounter,
		acceptCounter:  tcpAcceptCounter,
		rejectCounter:  tcpRejectCounter,
		responseTimer:  tcpResponseTimer,
		queryFilterer:  s.queryFilterer}
	udpDNShandler := &Handler{
		resolver:       &resolver,
		requestCounter: udpRequestCounter,
		acceptCounter:  udpAcceptCounter,
		rejectCounter:  udpRejectCounter,
		responseTimer:  udpResponseTimer,
		queryFilterer:  s.queryFilterer}

	udpHandler := dns.NewServeMux()
	tcpHandler := dns.NewServeMux()

	tcpHandler.HandleFunc(".", tcpDNShandler.Handle)
	udpHandler.HandleFunc(".", udpDNShandler.Handle)

	tcpServer := &dns.Server{Addr: s.Addr(),
		Net:          "tcp",
		Handler:      tcpHandler,
		ReadTimeout:  s.rTimeout,
		WriteTimeout: s.wTimeout}

	udpServer := &dns.Server{Addr: s.Addr(),
		Net:          "udp",
		Handler:      udpHandler,
		UDPSize:      65535,
		ReadTimeout:  s.rTimeout,
		WriteTimeout: s.wTimeout}

	go s.start(udpServer)
	go s.start(tcpServer)
}

func (s *Server) start(ds *dns.Server) {
	err := ds.ListenAndServe()
	if err != nil {
		logger.Fatalf("Start %s listener on %s failed:%s", ds.Net, s.Addr(), err.Error())
	}
}
