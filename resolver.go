package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "github.com/rcrowley/go-metrics"
    "strconv"
    "strings"
    "sync"
    "time"
)

type Resolver struct {
    etcd        *etcd.Client
    etcdPrefix  string
    defaultTtl  uint32
}

// GetFromStorage looks up a key in etcd and returns a slice of nodes. It supports two storage structures;
//  - File:         /foo/bar/.A -> "value"
//  - Directory:    /foo/bar/.A/0 -> "value-0"
//                  /foo/bar/.A/1 -> "value-1"
func (r *Resolver) GetFromStorage(key string) (nodes []*EtcdRecord, err error) {

    counter := metrics.GetOrRegisterCounter("resolver.etcd.query_count", metrics.DefaultRegistry)
    error_counter := metrics.GetOrRegisterCounter("resolver.etcd.query_error_count", metrics.DefaultRegistry)

    counter.Inc(1)
    debugMsg("Querying etcd for /" + r.etcdPrefix + key)

    response, err := r.etcd.Get(r.etcdPrefix + key, true, true)
    if err != nil {
        error_counter.Inc(1)
        return
    }

    var findKeys func(node *etcd.Node, ttl uint32, tryTtl bool)

    nodes = make([]*EtcdRecord, 0)
    findKeys = func(node *etcd.Node, ttl uint32, tryTtl bool) {
        if node.Dir == true {
            var lastValNode *etcd.Node
            for _, node := range node.Nodes {

                if strings.HasSuffix(node.Key, ".ttl") {
                    ttlValue, err := strconv.ParseUint(node.Value, 10, 32)
                    if err != nil {
                        debugMsg("Unable to convert ttl value to int: ", node.Value)
                    } else if lastValNode == nil {
                        debugMsg(".ttl node with no matching value node: ", node.Key)
                    } else {
                        findKeys(lastValNode, uint32(ttlValue), false)
                        lastValNode = nil
                        continue
                    }
                } else {
                    if lastValNode != nil {
                        findKeys(lastValNode, r.defaultTtl, false)
                    }
                    lastValNode = node
                }
            }

            if lastValNode != nil {
                findKeys(lastValNode, r.defaultTtl, false)
            }
        } else {
            // If for some reason this is passed a ttl node unexpectedly, bail
            if strings.HasSuffix(node.Key, ".ttl") {
                debugMsg("Unexpected .ttl node", node.Key)
                return
            }

            // If we don't have a TLL try and find one
            if tryTtl {
                ttlKey := node.Key + ".ttl"

                debugMsg("Querying etcd for " + ttlKey)
                response, err := r.etcd.Get(ttlKey, false, false)
                if err == nil {
                    ttlValue, err := strconv.ParseUint(response.Node.Value, 10, 32)
                    if err != nil {
                        debugMsg("Unable to convert ttl value to int: ", response.Node.Value)
                    } else {
                        ttl = uint32(ttlValue)
                    }
                }
            }

            nodes = append(nodes, &EtcdRecord{node, ttl})
        }
    }

    findKeys(response.Node, r.defaultTtl, true)

    return
}

// Authority returns a dns.RR describing the know authority for the given
// domain. It will recurse up the domain structure to find an SOA record that
// matches.
func (r *Resolver) Authority(domain string) (soa *dns.SOA) {
    tree := strings.Split(domain, ".")
    for i, _ := range tree {
        subdomain := strings.Join(tree[i:], ".")

        // Check for an SOA entry
        answers, err := r.LookupAnswersForType(subdomain, dns.TypeSOA)
        if err != nil {
            return
        }

        if len(answers) == 1 {
            soa = answers[0].(*dns.SOA)
            soa.Serial = uint32(time.Now().Truncate(time.Hour).Unix())
            return
        }
    }

    // Maintain a counter for when we don't have an authority for a domain.
    missing_counter := metrics.GetOrRegisterCounter("resolver.authority.missing_soa", metrics.DefaultRegistry)
    missing_counter.Inc(1)

    return
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

    answers := []dns.RR{}
    errors := []error{}
    errored := false

    var aChan chan dns.RR
    var eChan chan error

    if q.Qclass == dns.ClassINET {
        aChan, eChan = r.AnswerQuestion(q, true)
        answers, errors = gatherFromChannels(aChan, eChan)
    }

    errored = errored || len(errors) > 0
    if len(answers) == 0 {
        // If we failed to find any answers, let's keep looking up the tree for
        // any wildcard domain entries.
        parts := strings.Split(q.Name, ".")
        for level := 1; level < len(parts); level++ {
            domain := strings.Join(parts[level:], ".")
            if len(domain) > 1 {
                question := dns.Question{
                    Name: "*." + dns.Fqdn(domain),
                    Qtype: q.Qtype,
                    Qclass: q.Qclass}

                aChan, eChan = r.AnswerQuestion(question, true)
                answers, errors = gatherFromChannels(aChan, eChan)

                errored = errored || len(errors) > 0
                if len(answers) > 0 {
                    break;
                }
            }
        }
    }

    miss_counter := metrics.GetOrRegisterCounter("resolver.answers.miss", metrics.DefaultRegistry)
    hit_counter := metrics.GetOrRegisterCounter("resolver.answers.hit", metrics.DefaultRegistry)
    error_counter := metrics.GetOrRegisterCounter("resolver.answers.error", metrics.DefaultRegistry)

    if errored {
        // TODO(tarnfeld): Send special TXT records with a server error response code
        error_counter.Inc(1)
        msg.SetRcode(req, dns.RcodeServerFailure)
    } else if len(answers) == 0 {
        soa := r.Authority(q.Name)
        miss_counter.Inc(1)
        msg.SetRcode(req, dns.RcodeNameError)
        if soa != nil {
            msg.Ns = []dns.RR{soa}
        } else {
            msg.Authoritative = false // No SOA? We're not authoritative
        }
    } else {
        hit_counter.Inc(1)
        for _, rr := range answers {
            rr.Header().Name = q.Name
            msg.Answer = append(msg.Answer, rr)
        }
    }

    return
}

// Gather up results from answer and error channels into slices. Waits for the
// channels to be closed before returning.
func gatherFromChannels(rrsIn chan dns.RR, errsIn chan error) (rrs []dns.RR, errs []error) {
    rrs = []dns.RR{}
    errs = []error{}
    done := 0
    for done < 2 {
        select {
        case rr, ok := <-rrsIn:
            if ok {
                rrs = append(rrs, rr)
            } else {
                done++
            }
        case err, ok := <-errsIn:
            if ok {
                debugMsg("Caught error", err)
                errs = append(errs, err)
            } else {
                done++
            }
        }
    }
    return rrs, errs
}


// AnswerQuestion takes two channels, one for answers and one for errors. It will answer the
// given question writing the answers as dns.RR structures, and any errors it encounters along
// the way. The function will return immediately, and spawn off a bunch of goroutines
// to do the work, when using this function one should use a WaitGroup to know when all work
// has been completed.
func (r *Resolver) AnswerQuestion(q dns.Question, resolveAliases bool) (answers chan dns.RR, errors chan error) {
    answers = make(chan dns.RR)
    errors = make(chan error)

    typeStr := strings.ToLower(dns.TypeToString[q.Qtype])
    type_counter := metrics.GetOrRegisterCounter("resolver.answers.type." + typeStr, metrics.DefaultRegistry)
    type_counter.Inc(1)

    debugMsg("Answering question ", q)

    if q.Qtype == dns.TypeANY {
        wg := sync.WaitGroup{}
        wg.Add(len(convertersToRR))
        go func(){
            wg.Wait()
            close(answers)
            close(errors)
        }()
        for rrType, _ := range convertersToRR {
            go func(rrType uint16) {
                defer recover()
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
    } else if _, ok := convertersToRR[q.Qtype]; ok {
        go func() {
            defer func(){
                close(answers)
                close(errors)
            }()
            records, err := r.LookupAnswersForType(q.Name, q.Qtype)
            if err != nil {
                errors <- err
            } else {
                if len(records) > 0 {
                    for _, rr := range records {
                        answers <- rr
                    }
                } else if resolveAliases {
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
        }()
    } else {
        // nothing we can do
        close(answers)
        close(errors)
    }

    return answers, errors
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

        header := dns.RR_Header{Name: name, Class: dns.ClassINET, Rrtype: rrType, Ttl: node.ttl}
        answer, err := convertersToRR[rrType](node.node, header)

        if err != nil {
            debugMsg("Error converting type: ", err)
            return nil, err
        }

        answers[i] = answer
    }

    return
}

// NameExists will return true if the given domain name exists and has any
// resource records in the database. If an error occurs while querying for
// data the function will return false and an error.
func (r *Resolver) NameExists(name string) (exists bool, err error) {

    question := dns.Question{dns.Fqdn(name), dns.TypeANY, dns.ClassINET}
    aChan, eChan := r.AnswerQuestion(question, true)
    answers, errors := gatherFromChannels(aChan, eChan)

    if len(errors) > 0 {
        return false, errors[0]
    }
    return len(answers) > 0, nil
}

// RRSetExists returns true if RRs exist for the given name and type (value independent)
func (r *Resolver) RRSetExists(name string, rrType uint16) (exists bool, err error) {
    answers, err := r.LookupAnswersForType(dns.Fqdn(name), rrType)
    if err != nil {
        return false, err
    }

    return len(answers) > 0, nil
}

// RRSetMatches checks that the set of records in the DNS for the given name
// and type *exactly* match the given RRs: their data must match, and there must
// be no more or less RRs
func (r *Resolver) RRSetMatches(name string, rrType uint16, rrs []dns.RR) (matches bool, err error) {
    answers, err := r.LookupAnswersForType(dns.Fqdn(name), rrType)
    if err != nil {
        return false, err
    }
    if len(answers) != len(rrs) {
        return false, nil
    }
    matched := 0
    // I'm sure theres a neater/faster way than comparing all to all, but meh
    for _, rr := range rrs {
        for _, answer := range answers {
            // TTLS are explicitly excluded from comparison
            cmp := dns.Copy(answer)
            cmp.Header().Ttl = 0
            // TODO(orls): is string() enough? is there any relevant info not in the string reprs?
            if cmp.String() == rr.String() {
                matched++
                break
            }
        }
    }
    return matched == len(rrs), nil
}
