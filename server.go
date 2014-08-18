package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "github.com/rcrowley/go-metrics"
    "strconv"
    "time"
    "strings"
)

type QueryFilter struct {
    domain          string
    qTypes          []string
}

type Server struct {
    addr            string
    port            int
    etcd            *etcd.Client
    rTimeout        time.Duration
    wTimeout        time.Duration
    defaultTtl      uint32
    acceptFilters   []QueryFilter
    rejectFilters   []QueryFilter
}

type Handler struct {
    resolver        *Resolver
    acceptFilters    []QueryFilter
    rejectFilters    []QueryFilter

    // Metrics
    requestCounter      metrics.Counter
    responseTimer       metrics.Timer
}

func (h *Handler) Handle(response dns.ResponseWriter, req *dns.Msg) {
    h.requestCounter.Inc(1)
    h.responseTimer.Time(func() {
        debugMsg("Handling incoming query for domain " + req.Question[0].Name)

        // Figure out if the query matches any filters
        accepted := true
        for _, filter := range h.rejectFilters {
            if filter.Matches(req) {
                debugMsg("Rejected query")
                accepted = false
                break
            }
            debugMsg("Filter " + filter.domain + ":" + strings.Join(filter.qTypes, ",") + " not rejected")
        }
        if accepted {
            for _, filter := range h.acceptFilters {
                if !filter.Matches(req){
                    debugMsg("Filter " + filter.domain + ":" + strings.Join(filter.qTypes, ",") + " not accepted")
                    accepted = false
                    break
                }
                debugMsg("Filter " + filter.domain + ":" + strings.Join(filter.qTypes, ",") + " accepted")
            }
        }

        // Lookup the dns record for the request
        // This method will add any answers to the message
        var msg *dns.Msg
        if accepted != true {
            debugMsg("Query not accepted")
            msg =  new(dns.Msg)
            msg.SetReply(req)
            msg.SetRcode(req, dns.RcodeNameError)
            msg.Authoritative = false
            msg.RecursionAvailable = false
        } else {
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

    udpResponseTimer := metrics.NewTimer()
    metrics.Register("request.handler.udp.response_time", udpResponseTimer)
    udpRequestCounter := metrics.NewCounter()
    metrics.Register("request.handler.udp.requests", udpRequestCounter)

    resolver := Resolver{etcd: s.etcd, defaultTtl: s.defaultTtl}
    tcpDNShandler := &Handler{
        resolver: &resolver,
        requestCounter: tcpRequestCounter,
        responseTimer: tcpResponseTimer,
        acceptFilters: s.acceptFilters,
        rejectFilters: s.rejectFilters}
    udpDNShandler := &Handler{
        resolver: &resolver,
        requestCounter: udpRequestCounter,
        responseTimer: udpResponseTimer,
        acceptFilters: s.acceptFilters,
        rejectFilters: s.rejectFilters}

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

func (f *QueryFilter) Matches(req *dns.Msg) bool {
    queryDomain := req.Question[0].Name
    queryQType := dns.TypeToString[req.Question[0].Qtype]
    if !strings.HasSuffix(queryDomain, f.domain) {
        return false
    }

    matches := false
    for _, qType := range f.qTypes {
        if qType == queryQType {
            matches = true
        }
    }

    return matches
}
