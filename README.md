
discodns
======

A DNS *fowarder* and *nameserver* that first queries an [etcd](http://github.com/coreos/etcd) database of domains and records. It forwards requests it's not authoritative for onto a configured set of upstream nameservers (Google DNS by default).

The authoritative domains are configured using the `-domain` argument to the server, which switches the server from a *forwarder* to a *nameserver* for that domain zone. For example, `-domain=discodns.net.` will mean any domain queries within the `discodns.net.` zone will be served from the local database.

#### Key Features

- Full support for `CNAME` alias records
- Full support for both `NS` and `SOA` records for delegation either between discodns servers, or others.
- Full support for both IPv4 and IPv6 addresses
- Multiple resource records of different types per domain
- Support for recursive and non-recursive DNS queries

### Why is this useful?

We built discodns to be the backbone of our internal infrastructure, providing us with a robust and distributed Domain Name System to use. It allows us to register domains in any format, without imposing restrictions on how hosts/services must be named.

### Why etcd?

The choice was made as [etcd](http://github.com/coreos/etcd) is a simple, distributable k/v store with some very useful features that lend themselves well to solving the problem of service discovery. This DNS resolver is not designed to be the single point for discovering services throughout a network, but to make it easier. Services can utilize the same etcd cluster to both publish and subscribe to changes in the domain name system.

## Getting Started

### Building

The discodns project is written in Go, and uses an extensive library ([miekg/dns](https://github.com/miekg/dns)) to provide the actual implementation of the DNS protocol.

````shell
cd discodns
go get  # Ignore the error about installing discodns
make
````

### Running

It's as simple as launching the binary to start a DNS server listening on port 53 (tcp+udp) and accepting requests.

**Note:** You need to have an etcd cluster already running, use the `-h` argument for details on other configuration options.

````shell
cd discodns/build/
sudo ./bin/discodns -domain=discodns.net
````

### Try it out

It's incredibly easy to see your own domains come to life, simply insert a key for your record into etcd and then you're ready to go! Here we'll insert a custom `A` record for `discodns.net` pointing to `10.1.1.1`.

````shell
curl -L http://127.0.0.1:4001/v2/keys/net/discodns/.A/foobar -XPUT -d value="10.1.1.1"
{"action":"set","node":{"key":"/net/discodns/.A/foobar","value":"10.1.1.1","modifiedIndex":11,"createdIndex":11}}
````

````shell
; <<>> DiG 9.8.3-P1 <<>> @localhost discodns.net.
; .. truncated ..

;; QUESTION SECTION:
;discodns.net.            IN  A

;; ANSWER SECTION:
discodns.net.     0   IN  A   10.1.1.1
````

### Storage

The records are stored in a reverse domain structure, i.e `discodns.net` would equate to the key `com/discodns`. See the examples below;

- `discodns.net. -> A -> [10.1.1.1, 10.1.1.2]`
    - `/net/discodns/.A/foo -> 10.1.1.1`
    - `/net/discodns/.A/bar -> 10.1.1.2`

You'll notice the `.A` folder here on the end of the reverse domain, this signifies to the dns resolver that the values beneath are A records. You can have an infinite number of nested keys within this folder, allowing for some very interesting automation patterns. *Multiple keys within this folder represent multiple records for the same dns entry*.

````shell
; <<>> DiG 9.8.3-P1 <<>> @localhost discodns.net.
; .. truncated ..

;; QUESTION SECTION:
;discodns.net.            IN  A

;; ANSWER SECTION:
discodns.net.     0   IN  A   10.1.1.1
discodns.net.     0   IN  A   10.1.1.2
````

## Notes

Only a select few of record types are supported right now. These are listed here;

- `A` (ipv4)
- `AAAA` (ipv6)
- `TXT`
- `CNAME`
- `NS`
