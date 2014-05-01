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

type Resolver struct {
    etcd        *etcd.Client
    dns         *dns.Client
    domains     []string
    nameservers []string
    rTimeout    time.Duration
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
func (r *Resolver) Authority(domain string) []dns.RR {
    // tree := strings.Split(domain, ".")
    // for i, _ := range tree {
    //     subdomain := strings.Join(tree[i:], ".")
    //     answers, _ := r.LookupAnswersForType(subdomain, dns.TypeSOA)

    //     if len(answers) > 0 {
    //         for _, answer := range answers {
    //             answer.(*dns.SOA).Serial = uint32(time.Now().Truncate(time.Hour).Unix())
    //         }

    //         return answers
    //     }
    // }

    return make([]dns.RR, 0)
}

// IsAuthoritative will return true if this discodns server is authoritative
// for the given domain name.
func (r *Resolver) IsAuthoritative(name string) (bool) {
    for _, domain := range r.domains {
        if strings.HasSuffix(strings.ToLower(name), domain) {
            return true
        }
    }

    return false
}

// Lookup responds to DNS messages of type Query, with a dns message containing Answers.
// In the event that the query's value+type yields no known records, this falls back to
// querying the given nameservers instead.
func (r *Resolver) Lookup(req *dns.Msg) (msg *dns.Msg) {
    q := req.Question[0]

    msg = new(dns.Msg)
    msg.SetReply(req)
    msg.Authoritative = true
    msg.RecursionAvailable = true

    // Set the RD bit if we're going to use recursion, this helps the
    // resolver understand that we did actually use recursion, as they asked.
    if req.RecursionDesired {
        msg.RecursionDesired = true
    }

    wait := sync.WaitGroup{}
    answers := make(chan dns.RR)
    authorities := make(chan dns.RR)
    errors := make(chan error)

    if q.Qclass == dns.ClassINET {
        r.AnswerQuestion(req.RecursionDesired, answers, errors, authorities, q, &wait)
    }

    // Spawn a goroutine to close the channel as soon as all of the things
    // are done. This allows us to ensure we'll wait for all workers to finish
    // but allows us to collect up answers concurrently.
    go func() {
        wait.Wait()

        debugMsg("Finished processing all goroutines, closing channels")
        close(answers)
        close(authorities)
        close(errors)
    }()

    // Collect up all of the answers and any errors
    done := 0
    for done < 3 {
        select {
        case rr, ok := <-answers:
            if ok {
                msg.Answer = append(msg.Answer, rr)
            } else {
                done++
            }
        case authority, ok := <-authorities:
            if ok {
                msg.Ns = []dns.RR{authority}
            } else {
                done++
            }
        case err, ok := <-errors:
            if ok {
                debugMsg("Error")
                debugMsg(err)
            } else {
                done++
            }
        }
    }

    // If we've not found any answers
    if len(msg.Answer) == 0 {
        msg.SetRcode(req, dns.RcodeNameError)

        // If the domain query was within our authority, we need to send our SOA record
        if r.IsAuthoritative(q.Name) {
            msg.Ns = r.Authority(q.Name)
        }
    }

    return
}

// ResolveToIp accepts a string name (either an existing IPv4/6 address or a
// fully qualified domain) which is then resolved to a `net.IP` structure.
func (r *Resolver) ResolveToIP(name string) (ip net.IP) {
    debugMsg("Resolving " + name + " to IP")

    ip = net.ParseIP(name)
    if ip == nil {
        request := new(dns.Msg)
        request.Question = append(request.Question, dns.Question{
            Name: name,
            Qtype: dns.TypeA,
            Qclass: dns.ClassINET})

        // Let's enable recursion, if the name is not within the authority of
        // this server, we'll have to do much less effort.
        request.RecursionDesired = true

        result := r.Lookup(request)
        if len(result.Answer) > 0 {
            for _, answer := range result.Answer {
                if answer.Header().Rrtype == dns.TypeA {
                    ip = answer.(*dns.A).A
                    break
                }
            }
        }

        if !result.RecursionAvailable && ip == nil {
            // If the server didn't return any A records, and they told us they
            // didn't use recursion to find an answer, we'll have to iterate.
            // TODO(tarnfeld): We need to implement this. Pretty rare case, though.
        }
    }

    return
}

// QueryNameservers will query the given list of nameservers in parallel and
// return *the namesever that responds first*.
// 
// TODO(tarnfeld): This doesn't support iterative queries, if the query wants recursion
//                 but the server won't give it, iterate here.
func (r *Resolver) QueryNameservers(q *dns.Msg, ns []string) (msg *dns.Msg, err *error) {
    c := make(chan *dns.Msg)
    e := make(chan *error)
    for _, nameserver := range ns {
        go func(server string) {
            defer func() { recover() }()

            // Ensure we've got an IP address here, if not then try and resolve
            // the DNS name.
            ip := r.ResolveToIP(server)
            if ip != nil {
                msg, _, err := r.dns.Exchange(q, ip.String() + ":53")

                if err != nil {
                    e <- &err
                } else {
                    c <- msg
                }
            }
        }(nameserver)
    }

    // Wait for one of the nameservers to respond, or until we hit the configured timeout.
    timeout := time.NewTimer(r.rTimeout)
forever:
    for {
        select {
        case msg = <-c:
            timeout.Stop()
            break forever
        case err = <-e:
            timeout.Stop()
            break forever
        case <-timeout.C:
        }
    }

    // We close both the channels, this forces us to clear up any of the above goroutines
    // once they finished processing any work. We do this, because otherwise as soon
    // as this function returns, nothing can ever recv() on `c` or `e`.
    // This is a memory leak.
    close(c)
    close(e)

    return
}

// AnswerQuestion takes two channels, one for answers and one for errors. It will answer the
// given question writing the answers as dns.RR structures, and any errors it encounters along
// the way. The function will return immediately, and spawn off a bunch of goroutines
// to do the work, when using this function one should use a WaitGroup to know when all work
// has been completed.
func (r *Resolver) AnswerQuestion(recurse bool, answers chan dns.RR, errors chan error, authorities chan dns.RR, q dns.Question, wg *sync.WaitGroup) {

    debugMsg("Answering question ", q)
    if r.IsAuthoritative(q.Name) {
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
                defer func() { recover() }()
                defer wg.Done()

                records, err := r.LookupAnswersForType(q.Name, q.Qtype)
                if err != nil {
                    errors <- err
                } else {
                    // We're only going to recurse if;
                    //  - We've found no records
                    //  - The client has requested recursion
                    //  - The query is NOT for a CNAME or NS record
                    if len(records) > 0 {
                        for _, rr := range records {
                            answers <- rr
                        }
                    } else if recurse && q.Qtype != dns.TypeCNAME && q.Qtype != dns.TypeNS {

                        // Check for any aliases
                        debugMsg("Looking for CNAME records at " + q.Name)
                        cnames, err := r.LookupAnswersForType(q.Name, dns.TypeCNAME)
                        if err != nil {
                            errors <- err
                        } else if len(cnames) > 0 {
                            answers <- cnames[0] // Add the hop we've made

                            if cname, ok := cnames[0].(*dns.CNAME); ok {
                                // Kick off the recursive query
                                r.AnswerQuestion(true, answers, errors, authorities, dns.Question{
                                    Name: cname.Target,
                                    Qtype: q.Qtype,
                                    Qclass: q.Qclass}, wg)
                            }
                        } else {
                            // Check for delegated nameservers
                            tree := strings.Split(q.Name, ".")
                            for i, _ := range tree {
                                subdomain := strings.Join(tree[i:], ".")
                                debugMsg("Looking for NS records at " + subdomain)

                                ns, err := r.LookupAnswersForType(subdomain, dns.TypeNS)
                                if err != nil {
                                    errors <- err
                                } else if len(ns) > 0 {
                                    query := new(dns.Msg)
                                    query.SetQuestion(q.Name, q.Qtype)
                                    query.RecursionDesired = true

                                    nameservers := make([]string, 1)
                                    nameservers[0] = ns[0].(*dns.NS).Ns

                                    debugMsg("Forwarding request onto " + strings.Join(nameservers, " "))
                                    msg, err := r.QueryNameservers(query, nameservers)
                                    if err != nil {
                                        errors <- *err
                                    } else {
                                        for _, rr := range msg.Answer {
                                            answers <- rr
                                        }
                                        for _, soa := range msg.Ns {
                                            authorities <- soa
                                        }
                                    }

                                    break
                                }
                            }
                        }
                    }
                }
            }()
        }
    } else if recurse {
        // Hand off all other queries to the upstream nameservers
        // We're going to query all of them in a goroutine and listen to whoever is fastest
        // TODO(tarnfeld): We should maintain an RTT for each nameserver and go by that

        wg.Add(1)
        go func() {
            defer func() { recover() }()
            defer wg.Done()

            request := new(dns.Msg)
            request.RecursionDesired = recurse
            request.Question = []dns.Question{q}

            msg, err := r.QueryNameservers(request, r.nameservers)
            if err != nil {
                errors <- *err
            } else {
                for _, answer := range msg.Answer {
                    answers <- answer
                }
                for _, soa := range msg.Ns {
                    authorities <- soa
                }
            }
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
}
