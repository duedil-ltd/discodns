package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "strconv"
    "time"
)

type Server struct {
    addr        string
    port        int
    etcd        *etcd.Client
    ns          []string
    domain      string
    rTimeout    time.Duration
    wTimeout    time.Duration
}

type Handler struct {
    server  *Server
    net     string
    dns     *dns.Client
}

func (h *Handler) DNSClient() *dns.Client {
    if h.dns == nil {
        h.dns = &dns.Client{Net: h.net}
    }
    return h.dns
}

func (h *Handler) Handle(response dns.ResponseWriter, req *dns.Msg) {

    resolver := &Resolver{
        etcd: h.server.etcd,
        dns: h.DNSClient(),
        domain: h.server.domain,
        nameservers: h.server.ns,
        rTimeout: h.server.rTimeout,
    }

    // Lookup the dns record for the request
    // This method will add any answers to the message
    msg := resolver.Lookup(req)

    if msg != nil {
        response.WriteMsg(msg)
    }
}

func (s *Server) Addr() string {
    return s.addr + ":" + strconv.Itoa(s.port)
}

func (s *Server) Run() {

    tcpDNShandler := &Handler{server: s, net: "tcp"}

    tcpHandler := dns.NewServeMux()
    tcpHandler.HandleFunc(".", tcpDNShandler.Handle)

    udpDNShandler := &Handler{server: s, net: "udp"}

    udpHandler := dns.NewServeMux()
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
