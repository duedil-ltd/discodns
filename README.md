
etcdns
======

DueDil's internal DNS system for discovery of named systems. This project acts as a DNS server that proxies any public domain requests to the internet, but responds to `duedil.net.` queries. If a result is not found for a query, it will be forwarded to the internet.

When a `duedil.net.` query is received, the server then parses the query and looks for a relevant key in ZooKeeper that represents the domain. ZooKeeper is seen as the canonical store for all name information, and it is encouraged for systems that need to "find something" to use ZooKeeper instead of referencing DNS names.

## Getting Started

### Building

The etcdns project is written in Go, and uses an extensive library ([miekg/dns](https://github.com/miekg/dns)) to provide the actual implementation of the DNS protocol.

````shell
cd etcdns
make
````

### Running

It's as simple as launching the binary to start a DNS server listening on port 53 (tcp+udp) and accepting requests. At the very minimum you need to specify the location of your ZooKeeper cluster.

````shell
cd etcdns
sudo ./build/etcdns --zk zk://127.0.0.1:2181/dns
````

### Try it out

It's incredibly simple to see your own domains come to life, simply insert a key for your record into ZooKeeper (e.g using the Go client below) and then you can dig the server!

````go
package main

import (
    "time"
    "launchpad.net/gozk"
)

func main() {
    zk, session, err := gozk.Init("127.0.0.1:2181", time.Second)
    defer zk.Close()

    // Wait for connection.
    event := <-session
    if event.State != gozk.STATE_CONNECTED {
        println("Couldn't connect")
        return
    }

    _, err = zk.Create("/dns/net/duedil/foo/_A", "10.9.1.5", 0, gozk.WorldACL(gozk.PERM_ALL))
    _, err = zk.Create("/dns/net/duedil/foo/_TXT", "testing", 0, gozk.WorldACL(gozk.PERM_ALL))
}
````

````shell
dig @127.0.0.1 foo.duedil.net
; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 foo.duedil.net
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 64271
;; flags: qr rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available
 
;; QUESTION SECTION:
;foo.duedil.net.            IN  A
;foo.duedil.net.            IN  TXT
 
;; ANSWER SECTION:
foo.duedil.net.     1       IN  A   13.37.13.37
foo.duedil.net.     1       IN  TXT testing
 
;; Query time: 0 msec
;; SERVER: 127.0.0.1#53(127.0.0.1)
;; WHEN: Mon Mar 17 21:16:26 2014
;; MSG SIZE  rcvd: 54
````

## ZooKeeper Structure

Bleh bleh bleh.
