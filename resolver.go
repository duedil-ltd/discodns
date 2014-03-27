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
    etcd        *etcd.Client
    dns         *dns.Client
    rTimeout   time.Duration
}

// A resultsToRecords func converts raw values for etcd Nodes into dns Answers
type resultsToRecords func(rawRecords []*etcd.Node) (answers []dns.RR)

// a nodeToRecordMapper func turns a single 'file'-type etcd node into a dns resourcerecord
type nodeToRecordMapper func(node *etcd.Node) dns.RR

func nodeToIpAddr (node *etcd.Node) net.IP {

    ip := net.ParseIP(node.Value)
    if ip == nil {
        logger.Fatalf("Failed to parse IP value '%s'", node.Value)
    }

    return ip
}

// util function to run mapping function over everything in a list of etcd Nodes
func mapEachRecord(nodes []*etcd.Node, mapper nodeToRecordMapper) (answers []dns.RR) {

    answers = make([]dns.RR, len(nodes))

    for i := 0; i < len(nodes); i++ {
        answers[i] = mapper(nodes[i])
    }

    return
}

// search etcd for all records at a key )wether just one or a list from a 'directory'
func (r *Resolver) GetFromStorage(key string) (nodes []*etcd.Node) {

    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        logger.Printf("Error with etcd: %s", err)
        return
    }

    if response.Node.Dir == true {

        nodes = make([]*etcd.Node, len(response.Node.Nodes))
        for i := 0; i < len(response.Node.Nodes); i++ {
            nodes[i] = &response.Node.Nodes[i]
        }

    } else {

        nodes = make([]*etcd.Node, 1)
        nodes[0] = response.Node

    }

    return
}



func (r *Resolver) Lookup(req *dns.Msg, nameservers []string) (msg *dns.Msg) {
    q := req.Question[0]

    msg = new(dns.Msg)
    msg.SetReply(req)

    // Define some useful typical callbacks for core record types:

    // shorthand for a RR_Header ctor bound to name & class
    makeRRHeader := func (rrtype uint16) dns.RR_Header {
        return dns.RR_Header{Name: q.Name, Class: q.Qclass, Rrtype: rrtype, Ttl: 0}
    }

    // for some reason appending a slice of answers from mapEachRecord isn't
    // working, so add a util to loop and append (urgh)
    addAnswers := func (items []dns.RR) {
        for _, a := range items {
            msg.Answer = append(msg.Answer, a)
        }
    }

    // check query type and act accordingly

    if q.Qclass == dns.ClassINET {

        // A records
        if q.Qtype == dns.TypeA || q.Qtype == dns.TypeANY {

            nodes := r.GetFromStorage(nameToKey(q.Name, "/.A"))
            answers := mapEachRecord(nodes, func (node *etcd.Node) dns.RR {
                ip := nodeToIpAddr(node)
                return &dns.A{makeRRHeader(dns.TypeA), ip}
            })

            // not really sure why this doesnt work, `append` docs claim it should:
            // msg.Answer = append(msg.Answer, answers)
            addAnswers(answers)
        }

        // AAAA records
        if q.Qtype == dns.TypeAAAA || q.Qtype == dns.TypeANY {
            for _, a := range r.LookupAAAA(q.Name, q.Qclass) {
                msg.Answer = append(msg.Answer, a)
            }
        }

        // TXT records
        if q.Qtype == dns.TypeTXT || q.Qtype == dns.TypeANY {
            for _, a := range r.LookupTXT(q.Name, q.Qclass) {
                msg.Answer = append(msg.Answer, a)
            }
        }

        // CNAME records
        if q.Qtype == dns.TypeCNAME || q.Qtype == dns.TypeANY {
            for _, a := range r.LookupCNAME(q.Name, q.Qclass) {
                msg.Answer = append(msg.Answer, a)
            }
        }

        // NS records
        if q.Qtype == dns.TypeNS || q.Qtype == dns.TypeANY {
            for _, a := range r.LookupNS(q.Name, q.Qclass) {
                msg.Answer = append(msg.Answer, a)
            }
        }
    }

    if len(msg.Answer) == 0 {
        c := make(chan *dns.Msg)
        for _, nameserver := range nameservers {
            go r.LookupNameserver(c, req, nameserver)
        }

        timeout := time.After(r.rTimeout)
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


func (r *Resolver) LookupAAAA(name string, class uint16) (answers []*dns.AAAA) {
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
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: dns.TypeAAAA, Ttl: 0}
    answers[0] = &dns.AAAA{*rr_header, ip}

    return
}

func (r *Resolver) LookupTXT(name string, class uint16) (answers []*dns.TXT) {
    answers = make([]*dns.TXT, 0)

    key := nameToKey(name, "/.TXT")
    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        logger.Printf("Error with etcd: %s", err)
        return
    }

    node := response.Node

    answers = make([]*dns.TXT, 1)
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: dns.TypeTXT, Ttl: 0}
    answers[0] = &dns.TXT{*rr_header, []string{node.Value}}

    return
}

func (r *Resolver) LookupCNAME(name string, class uint16) (answers []*dns.CNAME) {
    answers = make([]*dns.CNAME, 0)

    key := nameToKey(name, "/.CNAME")
    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        logger.Printf("Error with etcd: %s", err)
        return
    }

    node := response.Node

    answers = make([]*dns.CNAME, 1)
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: dns.TypeCNAME, Ttl: 0}
    answers[0] = &dns.CNAME{*rr_header, node.Value}

    return
}

func (r *Resolver) LookupNS(name string, class uint16) (answers []*dns.NS) {
    answers = make([]*dns.NS, 0)

    key := nameToKey(name, "/.NS")
    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        logger.Printf("Error with etcd: %s", err)
        return
    }

    node := response.Node

    answers = make([]*dns.NS, 1)
    rr_header := &dns.RR_Header{Name: name, Class: class, Rrtype: dns.TypeNS, Ttl: 0}
    answers[0] = &dns.NS{*rr_header, node.Value}

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
