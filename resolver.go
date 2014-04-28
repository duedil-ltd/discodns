package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "net"
    "strings"
    "fmt"
    "bytes"
    "time"
    "sync"
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
    domain      string
    nameservers []string
    rTimeout    time.Duration
}

// GetFromStorage looks up a key in etcd and returns a slice of nodes. It supports two storage structures;
//  - File:         /foo/bar/.A -> "value"
//  - Directory:    /foo/bar/.A/0 -> "value-0"
//                  /foo/bar/.A/1 -> "value-1"
func (r *Resolver) GetFromStorage(key string) (nodes []*etcd.Node, err error) {

    response, err := r.etcd.Get(key, false, false)
    if err != nil {
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
// querying the given nameservers instead.
func (r *Resolver) Lookup(req *dns.Msg) (msg *dns.Msg) {
    q := req.Question[0]

    // Figure out if recursion is desired
    // If it is
    //  - We'll push any domains not under r.domain to the upstream resolvers
    //  - If the domain is a CNAME
    //    - If it points to something within r.domain we're going to resolve it
    //    - If it points to something outside we're going to resolve upstream
    // If not, we're not going to perform any further requests, this means
    //  - If the domain isn't within r.domain we ignore it completely
    //  - If the domain is a CNAME (not an A/AAAA) we're going to simply return it
    recurse := req.RecursionDesired

    msg = new(dns.Msg)
    msg.SetReply(req)
    msg.Authoritative = true
    msg.RecursionAvailable = true

    // We only handle domains we're authoritative for
    if strings.HasSuffix(strings.ToLower(q.Name), r.domain) {
        var err error
        if q.Qtype == dns.TypeANY {
            for rrType, _ := range converters {
                err = r.LookupAnswersForType(msg, q, rrType, false)
            }
        } else {
            if _, ok := converters[q.Qtype]; ok {
                err = r.LookupAnswersForType(msg, q, q.Qtype, recurse)
            }
        }

        // Handle any errors we might see when trying to do the lookup
        if err != nil {
            if e, ok := err.(*etcd.EtcdError); ok {
                if e.ErrorCode == 100 {
                    msg.SetRcode(req, dns.RcodeNameError)
                    return
                }
            }

            msg.SetRcode(req, dns.RcodeServerFailure)
            return
        }
    } else if recurse {
        // Hand off all other queries to the upstream nameservers
        // We're going to query all of them in a goroutine and listen to whoever is fastest
        // TODO(tarnfeld): We should maintain an RTT for each nameserver and go by that
        c := make(chan *dns.Msg)
        for _, nameserver := range r.nameservers {
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

func (r *Resolver) LookupAnswersForType(msg *dns.Msg, q dns.Question, rrType uint16, recurse bool) (err error) {
    name := strings.ToLower(q.Name)

    typeStr := dns.TypeToString[rrType]
    nodes, err := r.GetFromStorage(nameToKey(name, "/." + typeStr))

    // If there are no records found, and we're searching for A/AAAA let's look
    // for an alias (CNAME)
    cname := false
    if err != nil && err.(*etcd.EtcdError).ErrorCode == 100 {
        if (rrType == dns.TypeA || rrType == dns.TypeAAAA) {
            cname = true
            nodes, err = r.GetFromStorage(nameToKey(name, "/." + dns.TypeToString[dns.TypeCNAME]))
        }
    }

    // Process all of the nodes concurrently
    var wg sync.WaitGroup
    answers := make([][]dns.RR, len(nodes))
    for i, node := range nodes {
        wg.Add(1)

        go func(i int, node *etcd.Node) {
            if recurse && cname {
                header := dns.RR_Header{Name: q.Name, Class: q.Qclass, Rrtype: dns.TypeCNAME, Ttl: 0}
                answer, _ := converters[dns.TypeCNAME](node, header)
                if answer != nil {
                    answers[i] = append(answers[i], answer)
                }

                // Start a chain of recursive queries to find any leaf A records
                query := new(dns.Msg)
                query.SetQuestion(node.Value, rrType)
                query.RecursionDesired = true

                result := r.Lookup(query)
                if result != nil {
                    answers[i] = append(answers[i], result.Answer...)
                }
            } else {
                header := dns.RR_Header{Name: q.Name, Class: q.Qclass, Rrtype: rrType, Ttl: 0}
                answer, _ := converters[rrType](node, header)
                answers[i] = append(answers[i], answer)
            }

            wg.Done()
        }(i, node)
    }
    wg.Wait()

    // Collect up all of the answers
    for _, a := range answers {
        msg.Answer = append(msg.Answer, a...)
    }

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
