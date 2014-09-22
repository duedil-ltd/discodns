package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "fmt"
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
    
    rrsets := []dns.RR{req.Answer, req.Ns}
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

    // Ensure we recover from any panicking goroutine, this helps ensure we don't
    // leave any acquired locks around if possible
    defer func() {
        if r := recover(); r != nil {
            debugMsg("[PANIC] " + fmt.Sprint(r))
            msg.SetRcode(req, dns.RcodeServerFailure)
        }
    }()

    // Attempt to acquire a lock on all of the domains referenced in the update
    // If any lock attempt fails, all acquired locks will be released and no
    // update will be applied.
    for _, rrs := range rrsets {
        for _, rr := range rrs {
            lock := lockDomain(u.etcd, rr.Header().Name)
            defer lock.Unlock()
        }
    }

    // Validate the prerequisites of the update, returning immediately if they
    // are not satisfied.
    validationStatus := validatePrerequisites(req.Answer, u.resolver)
    if validationStatus != dns.RcodeSuccess {
        msg.SetRcode(req, validationStatus)
        return    
    }

    // Perform the updates to the domain name system
    // This is not inside any kind of transaction, so a failure here *can*
    // result in a partially updated zone.
    // TODO(tarnfeld): Figure out a way of rolling back changes, perhaps make
    // use of the etcd indexes?
    msg.SetRcode(req, performUpdate(req.Ns))

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
                if answers, ok := resolver.LookupAnswersForType(header.Name, dns.TypeANY); len(answers) > 0 {
                    if ok != nil {
                        return dns.RcodeServerFailure
                    }
                    return dns.RcodeNameError
                }
            } else {
                if answers, ok := resolver.LookupAnswersForType(header.Name, header.Rrtype); len(answers) > 0 {
                    if ok != nil {
                        return dns.RcodeServerFailure
                    }
                    return dns.RcodeNXRrset
                }
            }
        } else if header.Class == dns.ClassNone {
            if header.Rdlength != 0 {
                return dns.RcodeFormatError
            } else if header.Rrtype == dns.TypeANY {
                if answers, ok := resolver.LookupAnswersForType(header.Name, dns.TypeANY)); len(answers) == 0 {
                    if ok != nil {
                        return dns.RcodeServerFailure
                    }
                    return dns.RcodeYXDomain
                }
            } else {
                if answers, ok := resolver.LookupAnswersForType(header.Name, header.Rrtype)); len(answers) == 0 {
                    if ok != nil {
                        return dns.RcodeServerFailure
                    }
                    return dns.RcodeYXRrset
                }
            }
        } else if header.Class == dns.ClassINET {
            if answers, ok := resolver.LookupAnswersForType(header.Name, header.Rrtype); answers != rr {
                if ok != nil {
                    return dns.RcodeServerFailure
                }
                return false // TODO(tarnfekd): What error type should this be?
            }
        } else {
            return dns.RcodeFormatError
        }
    }

    return dns.RcodeSuccess
}

// performUpdate will commit the requested updates to the database
// It is assumed by this point all prerequisites have been validated and all
// domains are locked.
func performUpdate(rr []dns.RR) (rcode int) {
    return dns.RcodeSuccess
}

// nameInUser will return true if the name in the dns.RR_Header given is
// already in use, or false if not.
func nameInUse(header dns.RR_Header) []dns.RR {
    return false
}

// nameInUser will return true if the name AND type in the dns.RR_Header given
// is are already in use.
func nameAndTypeInUse(header dns.RR_Header) bool {
    return false
}
