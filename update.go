package main

import (
    "crypto/md5"
    "encoding/hex"
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "fmt"
    "strconv"
)

type DynamicUpdateManager struct {
    etcd        *etcd.Client
    etcdPrefix  string
    resolver    *Resolver
}

// Update will perform the necessary logic to update the DNS database with
// the changes described in the RFC-2136 formatted DNS message given.
// The return value will be the response message to send back to the client.
// It is assumed at this level the client has already authenticated and proven
// their right to update records in the given zone.
func (u *DynamicUpdateManager) Update(zone string, req *dns.Msg) (msg *dns.Msg) {

    rrsets := [][]dns.RR{req.Answer, req.Ns}
    msg = new(dns.Msg)
    msg.SetReply(req)

    // Verify the updates are within the zone we're modifying, since cross
    // zone updates are invalid.
    for _, rrs := range rrsets {
        for _, rr := range rrs {
            if dns.CompareDomainName(rr.Header().Name, zone) != dns.CountLabel(zone) {
                debugMsg("Domain " + rr.Header().Name + " is not in the " + zone + " zone")
                msg.SetRcode(req, dns.RcodeNotZone)
                return
            }
        }
    }

    // Ensure we recover from any panicking goroutine
    defer func() {
        if r := recover(); r != nil {
            debugMsg("[PANIC] " + fmt.Sprint(r))
            msg.SetRcode(req, dns.RcodeServerFailure)
        }
    }()

    // Attempt to acquire the dns-updates lock key.
    // TODO (orls): This means all updates from all running instances are
    // applied fully serially; this is less than ideal, the spec says they
    // should be serial only when conflicting with one another. But...this is
    // easier than building full transactions isolation mgmt :) For a low
    // frequency of updates, this should fine.
    lock := NewEtcdKeyLock(u.etcd, u.etcdPrefix + "._DISCODNS_UPDATE_LOCK")
    defer lock.Abandon()
    // block until locked or timed-out
    _, err := lock.WaitForAcquire(30)
    if err != nil {
        debugMsg("Failed to acquire or keep the update lock: ", err)
        msg.SetRcode(req, dns.RcodeServerFailure)
        return
    }

    // Validate the prerequisites of the update, returning immediately if they
    // are not satisfied.
    validationStatus := validatePrerequisites(req.Answer, u.resolver)
    if validationStatus != dns.RcodeSuccess {
        debugMsg("Validation of prerequisites failed")
        msg.SetRcode(req, validationStatus)
        return
    }

    // Perform the updates to the domain name system
    // This is not inside any kind of transaction, so a failure here *can*
    // result in a partially updated zone.
    // TODO(tarnfeld): Figure out a way of rolling back changes, perhaps make
    // use of the etcd indexes?
    msg.SetRcode(req, performUpdate(u.etcdPrefix, u.etcd, req.Ns))

    return
}

// validatePrerequisites will perform all necessary validation checks against
// update prerequisites and return the relevant status is validation fails,
// otherwise NOERROR(0) will be returned.
func validatePrerequisites(rr []dns.RR, resolver *Resolver) (rcode int) {
    for _, record := range rr {
        header := record.Header()
        if header.Ttl != 0 {
            return dns.RcodeFormatError
        }

        if header.Class == dns.ClassANY {
            if header.Rdlength != 0 {
                return dns.RcodeFormatError
            } else if header.Rrtype == dns.TypeANY {
                // RFC Meaning: "Name is in use"
                exists, err := resolver.NameExists(header.Name)
                if err != nil {
                    return dns.RcodeServerFailure
                }
                if !exists {
                    debugMsg("Domain that should exist does not ", header.Name)
                    return dns.RcodeNameError
                }
            } else {
                // RFC Meaning: "RRset exists (value independent)"
                if answers, ok := resolver.LookupAnswersForType(header.Name, header.Rrtype); len(answers) > 0 {
                    if ok != nil {
                        return dns.RcodeServerFailure
                    }
                    debugMsg("RRset that should exist does not ", header.Name)
                    return dns.RcodeNXRrset
                }
            }
        } else if header.Class == dns.ClassNONE {
            if header.Rdlength != 0 {
                return dns.RcodeFormatError
            } else if header.Rrtype == dns.TypeANY {
                // RFC Meaning: "Name is not in use"
                exists, err := resolver.NameExists(header.Name)
                if err != nil {
                    return dns.RcodeServerFailure
                }
                if exists {
                    debugMsg("Domain that should not exist does ", header.Name)
                    return dns.RcodeYXDomain
                }
            } else {
                // RFC meaning: "RRset does not exist"
                if answers, ok := resolver.LookupAnswersForType(header.Name, header.Rrtype); len(answers) == 0 {
                    if ok != nil {
                        return dns.RcodeServerFailure
                    }
                    debugMsg("RRset that should not exist does ", header.Name)
                    return dns.RcodeYXRrset
                }
            }
        } else if header.Class == dns.ClassINET {
            if header.Rrtype == dns.TypeANY {
                return dns.RcodeFormatError
            } else {
                // RFC Meaning: "RRset exists (value dependent)"
            }

            // TODO(tarnfeld): Perform strict comparisons between the resource records
            // if answers, ok := resolver.LookupAnswersForType(header.Name, header.Rrtype); answers != rr {
            //     if ok != nil {
            //         return dns.RcodeServerFailure
            //     }
            // }
        } else {
            return dns.RcodeFormatError
        }
    }

    return dns.RcodeSuccess
}

// performUpdate will commit the requested updates to the database
// It is assumed by this point all prerequisites have been validated and all
// domains are locked.
func performUpdate(prefix string, etcd *etcd.Client, records []dns.RR) (rcode int) {
    for _, rr := range records {
        header := rr.Header()
        if _, ok := convertersFromRR[header.Rrtype]; ok != true {
            panic("Record converter doesn't exist for " + dns.TypeToString[header.Rrtype])
        }

        node, err := convertRRToNode(rr, *header)
        if err != nil {
            panic("Got error when converting node")
        } else if node == nil {
            panic("Got NIL after successfully converting node")
        }

        // Prepend the etcd prefix, if we're given one
        node.Key = prefix + node.Key

        if header.Class == dns.ClassANY {
            debugMsg("Deleting all RRs from key " + node.Key)
            _, err := etcd.Delete(node.Key, true)
            if err != nil {
                debugMsg(err)
                panic("Failed to delete RRs from key " + node.Key)
            }

        } else if header.Class == dns.ClassNONE { // Delete an RR
            debugMsg("Delete specific RR: " + rr.String())
        } else { // Insert RR
            debugMsg("Inserting " + node.Value + " to " + node.Key)

            // Insert the record into etcd. Use MD5 of the node value as the
            // 'sub-key'. This makes duplicates impossible without sacrificing
            // TTL updates or extra pre-update lookup faff
            hasher := md5.New()
            hasher.Write([]byte(node.Value))
            subkey := hex.EncodeToString(hasher.Sum(nil))
            response, err := etcd.Set(node.Key + "/" + subkey, node.Value, 0)
            if err != nil {
                debugMsg(err)
                panic("Failed to insert record into etcd")
            }

            // Insert the TTL record if one has been requested
            if header.Ttl > 0 {
                ttl := strconv.FormatInt(int64(header.Ttl), 10)
                _, err = etcd.Set(response.Node.Key + ".ttl", ttl, 0)
                if err != nil {
                    debugMsg(err)
                    panic("Failed to insert ttl into etcd")
                }
            }
        }
    }

    return dns.RcodeSuccess
}
