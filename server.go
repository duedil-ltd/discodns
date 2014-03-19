package main

import (
	"github.com/coreos/go-etcd/etcd"
	"github.com/miekg/dns"
	"strconv"
	"time"
)

type Server struct {
	addr		string
	port		int
	etcd	 	etcd.Client
	rTimeout	time.Duration
	wTimeout	time.Duration
}

type Handler struct {
	server	*Server
}

func (h *Handler) Handle(response dns.ResponseWriter, req *dns.Msg) {

	q := req.Question[0]
	r := &Resolver{etcd: h.server.etcd}

    m := new(dns.Msg)
	m.SetReply(req)

	if q.Qclass == dns.ClassINET {
		if q.Qtype == dns.TypeA {
			logger.Printf("Q: A record for %s", q.Name)

			for _, a := range r.LookupA(q.Name, q.Qclass, q.Qtype) {
				header := a.Header()
				logger.Printf("A: %s (TTL %d)", a.A, header.Ttl)
				m.Answer = append(m.Answer, a)
			}
		}

		if q.Qtype == dns.TypeTXT {
			logger.Printf("Q: TXT record for %s", q.Name)

			for _, a := range r.LookupTXT(q.Name, q.Qclass, q.Qtype) {
				header := a.Header()
				logger.Printf("A: %s (TTL %d)", a.Txt[0], header.Ttl)
				m.Answer = append(m.Answer, a)
			}
		}

		if q.Qtype == dns.TypeCNAME {
			logger.Printf("Q: CNAME record for %s", q.Name)

			for _, a := range r.LookupCNAME(q.Name, q.Qclass, q.Qtype) {
				header := a.Header()
				logger.Printf("A: %s (TTL %d)", a.Target, header.Ttl)
				m.Answer = append(m.Answer, a)
			}
		}
	}

	response.WriteMsg(m)
}

func (s *Server) Addr() string {
	return s.addr + ":" + strconv.Itoa(s.port)
}

func (s *Server) Run() {

	handler := &Handler{server: s}

	tcpHandler := dns.NewServeMux()
	tcpHandler.HandleFunc(".", handler.Handle)

	udpHandler := dns.NewServeMux()
	udpHandler.HandleFunc(".", handler.Handle)

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
