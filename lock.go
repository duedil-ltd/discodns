package main

import (
    "code.google.com/p/go-uuid/uuid"
    "errors"
    "github.com/coreos/go-etcd/etcd"
    "time"
)

const (
    ETCD_LOCK_TTL       = 10
    ETCD_LOCK_HEARTBEAT = 5
)

// EtcdKeyLock represents a lock on a single key. Its semantics: once asked to
// Acquire, it will try to grab hold of the key in etcd, if it doesnt exist.
// If it does exist, then the lock waits for the other party to release it.
// Once Acquired, it will hold on to the lock indefinitely until Abandoned
type EtcdKeyLock struct {
    uuid       string
    key        string
    etcdClient *etcd.Client
    killChan   chan bool
    killed     bool
}

func NewEtcdKeyLock(etcdClient *etcd.Client, key string) *EtcdKeyLock {
    uuid := uuid.New()
    return &EtcdKeyLock{uuid: uuid, key: key, etcdClient: etcdClient, killChan: make(chan bool)}
}

// Start the process of trying to acquire a key lock. Returns a channel that
// will be sent true when the lock is aquired then closed. Callers can use this
// as their signal to proceed. The lock will kept indefinitely until abandoned.
func (l *EtcdKeyLock) Acquire() chan bool {
    inner_acq := make(chan bool)
    acquired := make(chan bool)
    go func() {
        _, ok := <-inner_acq
        if ok {
            acquired <- true
            go heartbeat(l, nil)
            go removeWhenCancelled(l)
        }
        close(acquired)
    }()
    go tryCreate(l, inner_acq)
    return acquired
}

// Abandons the lock. This just means closing the internal cancellation
// channel, causing all the child goros to do whatever they need to do.
func (l *EtcdKeyLock) Abandon() {
    if !l.killed {
        l.killed = true
        close(l.killChan)
    }
}

// Blocking version of Acquire, hiding the channels from callers who just want
// to synchronously wait
func (l *EtcdKeyLock) WaitForAcquire(timeout int) (bool, error) {
    timeoutKiller := time.AfterFunc(time.Duration(timeout) * time.Second, func(){
        l.Abandon()
    })
    acq := l.Acquire()
    ok, open := <- acq
    if ok && open {
        stopped := timeoutKiller.Stop()
        // Stopped == false means the timer already fired: this shouldn't be
        // possible (we shouldn't have been able to get an OK message in that
        // case). Erroring mostly out of paranoia: I'm positive this race can't
        // happen (famous last words though)
        if !stopped {
            return false, errors.New("Acquired a lock that was also killed by a timeout: this should not be possible!")
        }
        return true, nil
    } else {
        return false, errors.New("Couldn't aqcuire lock in time")
    }
}

// The internals of trying to get a lock: Try to PUT to the lock key iff it
// doesn't exist. If that suceeds, the lock is owned; signal the chan and
// return. If it fails, watch the etcd key until it changes. When it does
// change, try again. Repeat indefinitely until cancelled.
func tryCreate(l *EtcdKeyLock, acq chan bool) {
    defer close(acq)
    for {
        select {
        case _, chOpen := <-l.killChan:
            if !chOpen {
                return
            }
        default:
            _, err := l.etcdClient.Create(l.key, l.uuid, ETCD_LOCK_TTL)
            if err == nil {
                acq <- true
                return
            } else {
                err, cast := err.(*etcd.EtcdError)
                if cast && err.ErrorCode == 105 {
                    // Watch until it changes. (The current index is given to
                    // make sure we don't miss any changes in between)
                    _, err := l.etcdClient.Watch(l.key, err.Index+1, false, nil, l.killChan)
                    if err == nil {
                        // Skip the sleep and attempt a retry asap
                        continue
                    }
                }
                // if not created and not watching, pause briefly
                time.Sleep(1 * time.Second)
            }
        }
    }
}

func heartbeat(l *EtcdKeyLock, ping chan bool) {
    if ping == nil {
        ping = make(chan bool)
    }
    defer close(ping)
    for {
        time.Sleep(ETCD_LOCK_HEARTBEAT * time.Second)
        select {
        case _, chOpen := <-l.killChan:
            if !chOpen {
                return
            }
        default:
            _, err := l.etcdClient.Set(l.key, l.uuid, ETCD_LOCK_TTL)
            // non-blocking write on the ping channel
            select {
            case ping <- (err == nil):
            default:
            }
        }
    }
}

func removeWhenCancelled(l *EtcdKeyLock) {
    defer func() {
        l.etcdClient.CompareAndDelete(l.key, l.uuid, 0)
    }()
    for {
        _, chOpen := <-l.killChan
        if !chOpen {
            return
        }
    }
}
