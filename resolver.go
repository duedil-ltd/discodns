package main

import (
    "github.com/miekg/dns"
    "net"
)

type Resolver struct {
    zookeeper   string
}

func (r *Resolver) LookupA(name string) []dns.A {
    answers := make([]dns.A, 5)

    for i := 0; i < 5; i++ {
        rr_header := &dns.RR_Header{Ttl: 0}
        answers[i] = dns.A{*rr_header, net.ParseIP("13.37.13.37")}
    }
    
    return answers
}

func (r *Resolver) LookupTXT(req *dns.Msg, name string) []*dns.TXT {
    return nil
}

func (r *Resolver) LookupCNAME(req *dns.Msg, name string) []*dns.CNAME {
    return nil
}
