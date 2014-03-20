package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "net"
    "strings"
    "bytes"
    "time"
)

type Resolver struct {
    etcd    *etcd.Client
    dns     *dns.Client
}

func (r *Resolver) Lookup(req *dns.Msg, nameservers []string) (msg *dns.Msg) {
    q := req.Question[0]

    msg = new(dns.Msg)
    msg.SetReply(req)

    if q.Qclass == dns.ClassINET {

        // A records
        if q.Qtype == dns.TypeA {
            for _, a := range r.LookupA(q.Name, q.Qclass, q.Qtype) {
                msg.Answer = append(msg.Answer, a)
            }
        }

        // AAAA records
        if q.Qtype == dns.TypeAAAA {
            for _, a := range r.LookupAAAA(q.Name, q.Qclass, q.Qtype) {
                msg.Answer = append(msg.Answer, a)
            }
        }

        // TXT records
        if q.Qtype == dns.TypeTXT {
            for _, a := range r.LookupTXT(q.Name, q.Qclass, q.Qtype) {
                msg.Answer = append(msg.Answer, a)
            }
        }

        // CNAME records
        if q.Qtype == dns.TypeCNAME {
            for _, a := range r.LookupCNAME(q.Name, q.Qclass, q.Qtype) {
                msg.Answer = append(msg.Answer, a)
            }
        }
    }

    if len(msg.Answer) == 0 {
        c := make(chan *dns.Msg)
        for _, nameserver := range nameservers {
            go r.LookupNameserver(c, req, nameserver)
        }

        timeout := time.After(1 * time.Second)
        select {
        case result := <-c:
            return result
        case <-timeout:
            return
        }
    }

    return
}

func (r *Resolver) LookupNameserver(c chan *dns.Msg, req *dns.Msg, ns string) {
    msg, _, err := r.dns.Exchange(req, ns)
    if err != nil {
        return
    }
    c <- msg
}

func (r *Resolver) LookupA(name string, class uint16, rrtype uint16) (answers []*dns.A) {
    answers = make([]*dns.A, 0)

    key := nameToKey(name, "/.A")
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

func (r *Resolver) LookupAAAA(name string, class uint16, rrtype uint16) (answers []*dns.AAAA) {
    answers = make([]*dns.AAAA, 0)

    key := nameToKey(name, "/.AAAA")
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

    answers = make([]*dns.AAAA, 1)
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: rrtype, Ttl: 0}
    answers[0] = &dns.AAAA{*rr_header, ip}

    return
}

func (r *Resolver) LookupTXT(name string, class uint16, rrtype uint16) (answers []*dns.TXT) {
    answers = make([]*dns.TXT, 0)

    key := nameToKey(name, "/.TXT")
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

    key := nameToKey(name, "/.CNAME")
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
