package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "net"
    "strings"
    "fmt"
    "bytes"
    "time"
)

type NodeConversionError struct {
    Message string
    Node *etcd.Node
    AttemptedType uint16
}

func (e *NodeConversionError) Error() string {
    return fmt.Sprintf(
        "Unable to convert etc Node into a RR of type %d ('%s'): %s. Node details: %+v",
        e.AttemptedType,
        dns.TypeToString[e.AttemptedType],
        e.Message,
        &e.Node)
}

type Resolver struct {
    etcd        *etcd.Client
    dns         *dns.Client
    rTimeout   time.Duration
}

// GetFromStorage looks up a key in etcd and returns a slice of nodes. It supports two storage structures;
//  - File:         /foo/bar/.A -> "value"
//  - Directory:    /foo/bar/.A/0 -> "value-0"
//                  /foo/bar/.A/1 -> "value-1"
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

// Lookup responds to DNS messages of type Query, with a dns message containing Answers.
// In the event that the query's value+type yields no known records, this falls back to
// querying the given nameservers instead
func (r *Resolver) Lookup(req *dns.Msg, nameservers []string) (msg *dns.Msg) {
    q := req.Question[0]

    msg = new(dns.Msg)
    msg.SetReply(req)


    if q.Qclass == dns.ClassINET {
        if q.Qtype == dns.TypeANY {
            for rrType, _ := range converters {
                r.LookupAnswersForType(msg, q, rrType)
            }
        } else {
            if _, ok := converters[q.Qtype]; ok {
                r.LookupAnswersForType(msg, q, q.Qtype)
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

func (r *Resolver) LookupAnswersForType(msg *dns.Msg, q dns.Question, rrType uint16) {
    typeStr := dns.TypeToString[rrType]
    nodes := r.GetFromStorage(nameToKey(q.Name, "/." + typeStr))

    for _, node := range nodes {
        header := dns.RR_Header{Name: q.Name, Class: q.Qclass, Rrtype: rrType, Ttl: 0}
        answer, err := converters[rrType](node, header)
        if err != nil {
            logger.Println(err.Error())
        } else {
            msg.Answer = append(msg.Answer, answer)
        }
    }
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

// Map of conversion functions that turn individual etcd nodes into dns.RR answers
var converters = map[uint16]func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {

    dns.TypeA: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {

        ip := net.ParseIP(node.Value)
        if ip == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Failed to parse %s as IP Address", node.Value),
                AttemptedType: dns.TypeA,
            }
        } else if ip.To4() == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Value %s isn't an IPv4 address", node.Value),
                AttemptedType: dns.TypeA,
            }
        } else {
            rr = &dns.A{header, ip}
        }
        return
    },

    dns.TypeAAAA: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {

        ip := net.ParseIP(node.Value)
        if ip == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Failed to parse IP Address %s", node.Value),
                AttemptedType: dns.TypeAAAA,
            }
        } else if ip.To16() == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Value %s isn't an IPv6 address", node.Value),
                AttemptedType: dns.TypeA,
            }
        } else {
            rr = &dns.AAAA{header, ip}
        }
        return
    },

    dns.TypeTXT: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        rr = &dns.TXT{header, []string{node.Value}}
        return
    },

    dns.TypeCNAME: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        rr = &dns.CNAME{header, node.Value}
        return
    },

    dns.TypeNS: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        rr = &dns.NS{header, node.Value}
        return
    },
}
