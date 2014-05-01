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
    domains     []string
    rTimeout    time.Duration
    wTimeout    time.Duration
}

type Handler struct {
    net         string
    resolver    *Resolver
}

func (h *Handler) Handle(response dns.ResponseWriter, req *dns.Msg) {
    debugMsg("Handling incoming query for domain " + req.Question[0].Name)

    // Lookup the dns record for the request
    // This method will add any answers to the message
    msg := h.resolver.Lookup(req)
    if msg != nil {
        response.WriteMsg(msg)
    }

    response.Close()
    debugMsg("Sent response to ", response.RemoteAddr())
}

func (s *Server) Addr() string {
    return s.addr + ":" + strconv.Itoa(s.port)
}

func (s *Server) Run() {

    resolver := func (s *Server, net string) *Resolver {
        return &Resolver{
            etcd: s.etcd,
            dns: &dns.Client{
                Net: net,
                DialTimeout: s.rTimeout,
                ReadTimeout: s.rTimeout,
                WriteTimeout: s.wTimeout},
            domains: s.domains,
            nameservers: s.ns,
            rTimeout: s.rTimeout,
        }
    }

    tcpDNShandler := &Handler{resolver: resolver(s, "tcp")}
    udpDNShandler := &Handler{resolver: resolver(s, "udp")}

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
