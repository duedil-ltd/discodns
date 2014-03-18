package main

import (
    "github.com/miekg/dns"
    "net"
)

type Resolver struct {
    zookeeper   string
}

func (r *Resolver) LookupA(name string, class uint16, rrtype uint16) []*dns.A {
    answers := make([]*dns.A, 1)

    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: rrtype, Ttl: 0}
    answers[0] = &dns.A{*rr_header, net.ParseIP("13.37.13.37")}

    return answers
}

func (r *Resolver) LookupTXT(name string, class uint16, rrtype uint16) []*dns.TXT {
    answers := make([]*dns.TXT, 1)

    text := make([]string, 1)
    text[0] = "testing"

    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: rrtype, Ttl: 0}
    answers[0] = &dns.TXT{*rr_header, text}

    return answers
}

func (r *Resolver) LookupCNAME(name string, class uint16, rrtype uint16) []*dns.CNAME {
    answers := make([]*dns.CNAME, 1)

    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: rrtype, Ttl: 0}
    answers[0] = &dns.CNAME{*rr_header, "duedil.com."}

    return answers
}
