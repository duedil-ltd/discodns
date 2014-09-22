package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "github.com/rcrowley/go-metrics"
    "strconv"
    "time"
)

type Server struct {
    addr            string
    port            int
    etcd            *etcd.Client
    rTimeout        time.Duration
    wTimeout        time.Duration
    defaultTtl      uint32
    tsigSecret      map[string]string
    queryFilterer   *QueryFilterer
}

type Handler struct {
    resolver        *Resolver
    queryFilterer   *QueryFilterer
    updateManager   *DynamicUpdateManager

    // Metrics
    requestCounter      metrics.Counter
    acceptCounter       metrics.Counter
    rejectCounter       metrics.Counter
    // authFailCounter     metrics.Counter
    // authSuccessCounter  metrics.Counter
    responseTimer       metrics.Timer
}

func (h *Handler) Handle(response dns.ResponseWriter, req *dns.Msg) {
    h.requestCounter.Inc(1)
    h.responseTimer.Time(func() {
        debugMsg("Incoming message with opcode " + dns.OpcodeToString[req.MsgHdr.Opcode])

        var res *dns.Msg
        if req.MsgHdr.Opcode == dns.OpcodeQuery {
            // TODO(tarnfeld): Support for multiple questions?
            debugMsg("Handling incoming query for domain " + req.Question[0].Name)

            // Lookup the dns record for the request
            // This method will add any answers to the message
            if h.queryFilterer.ShouldAcceptQuery(req) != true {
                debugMsg("Query not accepted")

                h.rejectCounter.Inc(1)

                res = new(dns.Msg)
                res.SetReply(req)
                res.SetRcode(req, dns.RcodeNameError)
                res.Authoritative = true
                res.RecursionAvailable = false

                // Add a useful TXT record
                header := dns.RR_Header{Name: req.Question[0].Name,
                                        Class: dns.ClassINET,
                                        Rrtype: dns.TypeTXT}
                res.Ns = []dns.RR{&dns.TXT{header, []string{"Rejected query based on matched filters"}}}
            } else {
                h.acceptCounter.Inc(1)
                res = h.resolver.Lookup(req)
            }
        } else if req.MsgHdr.Opcode == dns.OpcodeUpdate {
            zone := req.Question[0].Name
            debugMsg("Handling incoming update for zone " + zone)

            res = new(dns.Msg)
            res.SetReply(req)

            // Authenticate the request
            if req.IsTsig() != nil && response.TsigStatus() == nil {
                sig := req.IsTsig()
                debugMsg("Authenticated update request")
                
                // Verify the tsig is for the correct zone
                if sig.Hdr.Name != zone {
                    res.SetRcode(req, dns.RcodeBadSig)
                } else {
                    res = h.updateManager.Update(zone, req)
                }
            } else {
                debugMsg("Authentication failed")
                res.SetRcode(req, dns.RcodeNotAuth)
            }
        } else {
            res = new(dns.Msg)
            res.SetReply(req)
            res.SetRcode(req, dns.RcodeNotImplemented)
        }

        if res != nil {
            err := response.WriteMsg(res)
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
    updateManager := DynamicUpdateManager{etcd: s.etcd, resolver: &resolver}
    tcpDNShandler := &Handler{
        resolver: &resolver,
        requestCounter: tcpRequestCounter,
        acceptCounter: tcpAcceptCounter,
        rejectCounter: tcpRejectCounter,
        responseTimer: tcpResponseTimer,
        queryFilterer: s.queryFilterer,
        updateManager: &updateManager}
    udpDNShandler := &Handler{
        resolver: &resolver,
        requestCounter: udpRequestCounter,
        acceptCounter: udpAcceptCounter,
        rejectCounter: udpRejectCounter,
        responseTimer: udpResponseTimer,
        queryFilterer: s.queryFilterer,
        updateManager: &updateManager}

    udpHandler := dns.NewServeMux()
    tcpHandler := dns.NewServeMux()

    tcpHandler.HandleFunc(".", tcpDNShandler.Handle)
    udpHandler.HandleFunc(".", udpDNShandler.Handle)

    tcpServer := &dns.Server{Addr: s.Addr(),
        Net:          "tcp",
        Handler:      tcpHandler,
        ReadTimeout:  s.rTimeout,
        WriteTimeout: s.wTimeout,
        TsigSecret:   s.tsigSecret}

    udpServer := &dns.Server{Addr: s.Addr(),
        Net:          "udp",
        Handler:      udpHandler,
        UDPSize:      65535,
        ReadTimeout:  s.rTimeout,
        WriteTimeout: s.wTimeout,
        TsigSecret:   s.tsigSecret}

    go s.start(udpServer)
    go s.start(tcpServer)
}

func (s *Server) start(ds *dns.Server) {
    err := ds.ListenAndServe()
    if err != nil {
        logger.Fatalf("Start %s listener on %s failed:%s", ds.Net, s.Addr(), err.Error())
    }
}
