package main

import (
    "bytes"
    "fmt"
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "net"
    "strconv"
    "strings"
    "sync"
    "time"
)

type Resolver struct {
    etcd        *etcd.Client
}

// GetFromStorage looks up a key in etcd and returns a slice of nodes. It supports two storage structures;
//  - File:         /foo/bar/.A -> "value"
//  - Directory:    /foo/bar/.A/0 -> "value-0"
//                  /foo/bar/.A/1 -> "value-1"
func (r *Resolver) GetFromStorage(key string) (nodes []*etcd.Node, err error) {

    debugMsg("Querying etcd for " + key)
    response, err := r.etcd.Get(key, false, false)
    if err != nil {
        return
    }

    if response.Node.Dir == true {
        // TODO(orls): Does this need to convert to a slice?
        nodes = make([]*etcd.Node, len(response.Node.Nodes))
        for i := 0; i < len(response.Node.Nodes); i++ {
            nodes[i] = response.Node.Nodes[i]
        }
    } else {
        nodes = make([]*etcd.Node, 1)
        nodes[0] = response.Node
    }

    return
}

// Authority returns a dns.RR describing the know authority for the given
// domain. It will recurse up the domain structure to find an SOA record that
// matches.
func (r *Resolver) Authority(domain string) ([]dns.RR, *dns.SOA) {
    tree := strings.Split(domain, ".")
    for i, _ := range tree {
        subdomain := strings.Join(tree[i:], ".")

        // Check for an SOA entry
        answers, err := r.LookupAnswersForType(subdomain, dns.TypeSOA)
        if err != nil {
            return make([]dns.RR, 0), &dns.SOA{}
        }

        if len(answers) == 1 {
            soa := answers[0].(*dns.SOA)
            soa.Serial = uint32(time.Now().Truncate(time.Hour).Unix())

            nameservers, err := r.LookupAnswersForType(subdomain, dns.TypeNS)
            if err != nil {
                return make([]dns.RR, 0), &dns.SOA{}
            }

            return nameservers, soa
        }
    }

    return make([]dns.RR, 0), &dns.SOA{}
}

// Lookup responds to DNS messages of type Query, with a dns message containing Answers.
// In the event that the query's value+type yields no known records, this falls back to
// querying the given nameservers instead.
func (r *Resolver) Lookup(req *dns.Msg) (msg *dns.Msg) {
    q := req.Question[0]

    msg = new(dns.Msg)
    msg.SetReply(req)
    msg.Authoritative = true
    msg.RecursionAvailable = false // We're a nameserver, no recursion for you!

    wait := sync.WaitGroup{}
    answers := make(chan dns.RR)
    errors := make(chan error)

    if q.Qclass == dns.ClassINET {
        r.AnswerQuestion(answers, errors, q, &wait)
    }

    // Spawn a goroutine to close the channel as soon as all of the things
    // are done. This allows us to ensure we'll wait for all workers to finish
    // but allows us to collect up answers concurrently.
    go func() {
        wait.Wait()

        debugMsg("Finished processing all goroutines, closing channels")
        close(answers)
        close(errors)
    }()

    // Collect up all of the answers and any errors
    done := 0
    for done < 2 {
        select {
        case rr, ok := <-answers:
            if ok {
                msg.Answer = append(msg.Answer, rr)
            } else {
                done++
            }
        case err, ok := <-errors:
            if ok {
                // TODO(tarnfeld): Send special TXT records with a server error response code
                debugMsg("Error")
                debugMsg(err)
            } else {
                done++
            }
        }
    }

    // Send the correct authority records
    nameservers, soa := r.Authority(q.Name)
    if len(msg.Answer) == 0 {
        msg.SetRcode(req, dns.RcodeNameError)
        if soa != nil{
            msg.Ns = []dns.RR{soa}
        }
    } else {
        msg.Ns = nameservers
    }

    return
}

// AnswerQuestion takes two channels, one for answers and one for errors. It will answer the
// given question writing the answers as dns.RR structures, and any errors it encounters along
// the way. The function will return immediately, and spawn off a bunch of goroutines
// to do the work, when using this function one should use a WaitGroup to know when all work
// has been completed.
func (r *Resolver) AnswerQuestion(answers chan dns.RR, errors chan error, q dns.Question, wg *sync.WaitGroup) {

    debugMsg("Answering question ", q)
    if q.Qtype == dns.TypeANY {
        wg.Add(len(converters))

        for rrType, _ := range converters {
            go func(rrType uint16) {
                defer func() { recover() }()
                defer wg.Done()

                results, err := r.LookupAnswersForType(q.Name, rrType)
                if err != nil {
                    errors <- err
                } else {
                    for _, answer := range results {
                        answers <- answer
                    }
                }
            }(rrType)
        }
    } else if _, ok := converters[q.Qtype]; ok {
        wg.Add(1)

        go func() {
            records, err := r.LookupAnswersForType(q.Name, q.Qtype)
            if err != nil {
                errors <- err
            } else {
                if len(records) > 0 {
                    for _, rr := range records {
                        answers <- rr
                    }
                } else {
                    cnames, err := r.LookupAnswersForType(q.Name, dns.TypeCNAME)
                    if err != nil {
                        errors <- err
                    } else {
                        if len(cnames) > 1 {
                            errors <- &RecordValueError{
                                Message: "Multiple CNAME records is invalid",
                                AttemptedType: dns.TypeCNAME}
                        } else if len(cnames) > 0 {
                            answers <- cnames[0]
                        }
                    }
                }
            }

            wg.Done()
        }()
    }
}

func (r *Resolver) LookupAnswersForType(name string, rrType uint16) (answers []dns.RR, err error) {
    name = strings.ToLower(name)

    typeStr := dns.TypeToString[rrType]
    nodes, err := r.GetFromStorage(nameToKey(name, "/." + typeStr))

    if err != nil {
        if e, ok := err.(*etcd.EtcdError); ok {
            if e.ErrorCode == 100 {
                return answers, nil
            }
        }

        return
    }

    answers = make([]dns.RR, len(nodes))
    for i, node := range nodes {

        // TODO(tarnfeld): TTL 0 - make this configurable
        header := dns.RR_Header{Name: name, Class: dns.ClassINET, Rrtype: rrType, Ttl: 0}
        answer, err := converters[rrType](node, header)

        if err != nil {
            return nil, err
        }

        answers[i] = answer
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
        rr = &dns.CNAME{header, dns.Fqdn(node.Value)}
        return
    },

    dns.TypeNS: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        rr = &dns.NS{header, dns.Fqdn(node.Value)}
        return
    },

    dns.TypeSOA: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        parts := strings.SplitN(node.Value, "\\t", 6)

        refresh, err := strconv.ParseUint(parts[2], 10, 32)
        if err != nil {
            return nil, err
        }

        retry, err := strconv.ParseUint(parts[3], 10, 32)
        if err != nil {
            return nil, err
        }

        expire, err := strconv.ParseUint(parts[4], 10, 32)
        if err != nil {
            return nil, err
        }

        minttl, err := strconv.ParseUint(parts[5], 10, 32)
        if err != nil {
            return nil, err
        }

        rr = &dns.SOA{
            Hdr:     header,
            Ns:      dns.Fqdn(parts[0]),
            Mbox:    dns.Fqdn(parts[1]),
            Refresh: uint32(refresh),
            Retry:   uint32(retry),
            Expire:  uint32(expire),
            Minttl:  uint32(minttl)}

        return
    },
}
