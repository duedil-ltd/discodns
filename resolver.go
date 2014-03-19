package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "net"
    "strings"
    "bytes"
)

type Resolver struct {
    etcd    etcd.Client
}

func (r *Resolver) LookupA(name string, class uint16, rrtype uint16) (answers []*dns.A) {
    answers = make([]*dns.A, 0)

    key := nameToKey(name, "/_A")
    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        logger.Printf("Error with etcd: %s", err)
        return
    }

    node := response.Node

    ip := net.ParseIP(node.Value)
    if ip == nil {
        logger.Fatalf("Failed to parse IP value '%s'", node.Value)
    }

    answers = make([]*dns.A, 1)
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: rrtype, Ttl: 0}
    answers[0] = &dns.A{*rr_header, ip}

    return
}

func (r *Resolver) LookupTXT(name string, class uint16, rrtype uint16) (answers []*dns.TXT) {
    answers = make([]*dns.TXT, 0)

    key := nameToKey(name, "/_TXT")
    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        logger.Printf("Error with etcd: %s", err)
        return
    }

    node := response.Node

    answers = make([]*dns.TXT, 1)
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: rrtype, Ttl: 0}
    answers[0] = &dns.TXT{*rr_header, []string{node.Value}}

    return
}

func (r *Resolver) LookupCNAME(name string, class uint16, rrtype uint16) (answers []*dns.CNAME) {
    answers = make([]*dns.CNAME, 0)

    key := nameToKey(name, "/_CNAME")
    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        logger.Printf("Error with etcd: %s", err)
        return
    }

    node := response.Node

    answers = make([]*dns.CNAME, 1)
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: rrtype, Ttl: 0}
    answers[0] = &dns.CNAME{*rr_header, node.Value}

    return
}

func nameToKey(name string, suffix string) string {
    segments := strings.Split(name, ".")

    var keyBuffer bytes.Buffer
    for i := len(segments) - 1; i >= 0; i-- {
        if len(segments[i]) > 0 {
            keyBuffer.WriteString("/")
            keyBuffer.WriteString(segments[i])
        }
    }

    keyBuffer.WriteString(suffix)
    return keyBuffer.String()
}
