package main

import (
    "github.com/coreos/go-etcd/etcd"
)

type DomainLock struct {
    etcd        *etcd.Client
    domain      string
    index       uint64
    lockPath    string
    lockExpiry  uint64
}

func (l *DomainLock) Lock(shouldPanic bool) error {
    if l.index > 0 || l.lockPath != "" {
        return nil
    }

    l.lockPath = nameToKey(l.domain, "/._UPDATE_LOCK")
    debugMsg("Locking " + l.domain + " at " + l.lockPath)

    response, err := l.etcd.Create(l.lockPath, "", l.lockExpiry)
    if err != nil {
        debugMsg("Failed to acquire lock on domain " + l.domain)
        debugMsg(err)

        if shouldPanic {
            panic("Failed to acquire lock on domain " + l.domain)
        }

        return err
    }

    l.index = response.Node.CreatedIndex
    return nil
}

func (l *DomainLock) Unlock(shouldPanic bool) error {
    debugMsg("Unlocking " + l.domain + " from " + l.lockPath)

    _, err := l.etcd.CompareAndDelete(l.lockPath, "", l.index)
    if err != nil {
        debugMsg("Failed to unlock domain " + l.domain)
        debugMsg(err)

        if shouldPanic {
            panic("Failed to unlock domain " + l.domain)
        }
    }

    return err
}

// IsLocked will return true if the lock is still acquired by this instance
// Since locks may have an expiry, it is possible for the lock to expire and be
// acquired by another party
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

// lockDomain will lock the given domain in the given etcd cluster, and return
// the DomainLock struct pre-populated such that calling Domain Unlock()
// will release the lock.
func lockDomain(etcd *etcd.Client, domain string) (lock *DomainLock) {
    lock = &DomainLock{etcd: etcd, domain: domain, lockExpiry: 30}
    defer lock.Lock(true)
    return lock
}
