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
    authority   string
    rTimeout    time.Duration
    wTimeout    time.Duration
}

type Handler struct {
    net         string
    resolver    *Resolver
}

func (h *Handler) Handle(response dns.ResponseWriter, req *dns.Msg) {
    // Lookup the dns record for the request
    // This method will add any answers to the message
    msg := h.resolver.Lookup(req)

    if msg != nil {
        response.WriteMsg(msg)
    }
}

func (s *Server) Addr() string {
    return s.addr + ":" + strconv.Itoa(s.port)
}

func (s *Server) Run() {

    resolver := func (s *Server, client *dns.Client) *Resolver {
        return &Resolver{
            etcd: s.etcd,
            dns: client,
            domain: s.domain,
            nameservers: s.ns,
            authority: s.authority,
            rTimeout: s.rTimeout,
        }
    }

    tcpDNShandler := &Handler{resolver: resolver(s, &dns.Client{Net: "tcp"})}
    udpDNShandler := &Handler{resolver: resolver(s, &dns.Client{Net: "udp"})}

    udpHandler := dns.NewServeMux()
    tcpHandler := dns.NewServeMux()

    // TODO(tarnfeld): Perhaps we could move up resolution of "." to here and
    //                 specifically only call our handler for s.domain?
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
