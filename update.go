package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "fmt"
)

type DynamicUpdateManager struct {
    etcd        *etcd.Client
    etcdPrefix  string
}

func (u *DynamicUpdateManager) Update(zone string, req *dns.Msg) (msg *dns.Msg) {
    msg = new(dns.Msg)
    msg.SetReply(req)

    // Verify the updates are within the zone
    for _, rr := range req.Answer {
        if dns.CompareDomainName(rr.Header().Name, zone) != dns.CountLabel(zone) {
            msg.SetRcode(req, dns.RcodeNotZone)
            return msg
        }
    }
    for _, rr := range req.Ns {
        if dns.CompareDomainName(rr.Header().Name, zone) != dns.CountLabel(zone) {
            msg.SetRcode(req, dns.RcodeNotZone)
            return msg
        }
    }

    // Ensure we recover from any panic to help ensure we unlock the locked domains
    defer func() {
        if r := recover(); r != nil {
            debugMsg("[PANIC] " + fmt.Sprint(r))
            msg.SetRcode(req, dns.RcodeServerFailure)
        }
    }()

    // Attempt to acquire a lock on all of the domains referenced in the update
    // If any lock attempt fails, all acquired locks will be released and no
    // update will be applied.
    for _, rr := range req.Answer {
        lock := lockDomain(u.etcd, rr.Header().Name)
        defer lock.Unlock()
    }
    for _, rr := range req.Ns {
        lock := lockDomain(u.etcd, rr.Header().Name)
        defer lock.Unlock()
    }

    // Validate the prerequisites of the update request
    validationStatus := validatePrerequisites(req.Answer)
    msg.SetRcode(req, validationStatus)

    // If we failed to validate prerequisites, return the error
    if validationStatus != dns.RcodeSuccess {
        return msg
    }

    // Perform the updates to the domain name system
    msg.SetRcode(req, performUpdate(req.Ns))

    return
}

// validatePrerequisites will perform all necessary validation checks against
// update prerequisites and return the relevent status is validation fails,
// otherwise NOERROR(0) will be returned.
func validatePrerequisites(rr []dns.RR) (rcode int) {
    for _, record := range rr {
        header := record.Header()
        if header.Ttl != 0 {
            return dns.RcodeFormatError
        }

        if header.Class == dns.ClassANY {
            if header.Rdlength != 0 {
                return dns.RcodeFormatError
            }
            if header.Rrtype == dns.TypeANY {
                // TODO (dns.TypeANY, header.Name) dns.RcodeNameError
            } else {
                // TODO (header.Rrtype, header.Name) dns.RcodeNXRrset
            }
        } else if header.Class == dns.ClassNone {
            if header.Rdlength != 0 {
                return dns.RcodeFormatError
            }
            if header.Rrtype == dns.TypeANY {
                // TODO (dns.TypeANY, header.Name) dns.RcodeYXDomain
            } else {
                // TODO (header.Rrtype, header.Name) dns.RcodeYXRrset
            }
        } else if header.Class == dns.ClassINET {
            // Compare rr with the same type+name in the db (value comparison)
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

type DomainLock struct {
    etcd        *etcd.Client
    domain      string
    index       uint64
    lockPath    string
    lockExpiry  uint64
}

func (l *DomainLock) Unlock() error {
    debugMsg("Unlocking " + l.domain + " from " + l.lockPath)

    _, err := l.etcd.CompareAndDelete(l.lockPath, "", l.index)
    if err != nil {
        debugMsg(err)
    }

    return err
}

func (l *DomainLock) Lock() error {
    if l.index > 0 || l.lockPath != "" {
        return nil
    }

    l.lockPath = nameToKey(l.domain, "/._UPDATE_LOCK")
    debugMsg("Locking " + l.domain + " at " + l.lockPath)

    response, err := l.etcd.Create(l.lockPath, "", l.lockExpiry)
    if err != nil {
        panic("Failed to acquire lock on domain " + l.domain)
    }

    l.index = response.Node.CreatedIndex
    return err
}

// IsLocked will return true if 
func (l *DomainLock) IsLocked() (locked bool) {
    locked = false

    // Verify that the domain is locked and that it's *our* lock
    if l.lockPath != "" {
        response, err := l.etcd.Get(l.lockPath, false, false)
        if err == nil {
            locked = response.Node.CreatedIndex == l.index
        }
    }

    return
}

func lockDomain(etcd *etcd.Client, domain string) (lock *DomainLock) {
    lock = &DomainLock{etcd: etcd, domain: domain, lockExpiry: 30}
    defer lock.Lock()
    return lock
}
