
discodns
======

A DNS *fowarder* and *nameserver* that first queries an [etcd](http://github.com/coreos/etcd) database of domains and records. It forwards requests it's not authoritative for onto a configured set of upstream nameservers (Google DNS by default).

The authoritative domains are configured using the `--domain` argument to the server, which switches the server from a *forwarder* to a *nameserver* for that domain zone. For example, `--domain=discodns.net.` will mean any domain queries within the `discodns.net.` zone will be served from the local database.

#### Key Features

- Full support for `CNAME` alias records
- Full support for both `NS` and `SOA` records for delegation either between discodns servers, or others.
- Full support for both IPv4 and IPv6 addresses
- Multiple resource records of different types per domain
- Support for recursive and non-recursive DNS queries

#### Coming Soon

- Metrics (can be shipped to statsd/graphite)
- Support for wildcard domains
- Support for zone transfers (`AXFR`), though enabling these would be on a short-lived basis

### Why is this useful?

We built discodns to be the backbone of our internal infrastructure, providing us with a robust and distributed Domain Name System to use. It allows us to register domains in any format, without imposing restrictions on how hosts/services must be named.

### Why etcd?

The choice was made as [etcd](http://github.com/coreos/etcd) is a simple, distributable k/v store with some very useful features that lend themselves well to solving the problem of service discovery. This DNS resolver is not designed to be the single point for discovering services throughout a network, but to make it easier. Services can utilize the same etcd cluster to both publish and subscribe to changes in the domain name system.

## Getting Started

### Building

The discodns project is written in Go, and uses an extensive library ([miekg/dns](https://github.com/miekg/dns)) to provide the actual implementation of the DNS protocol.

````shell
cd discodns
make
````

### Running

It's as simple as launching the binary to start a DNS server listening on port 53 (tcp+udp) and accepting requests.

**Note:** You need to have an etcd cluster already running, use the `-h` argument for details on other configuration options.

````shell
cd discodns/build/
sudo ./bin/discodns --domain=discodns.net --ns=8.8.8.8 --ns=8.8.4.4
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

### Authority

If you're not familiar with the DNS specification, to support correct DNS Delegation using `NS` records, each top level domain needs to have it's own `SOA` record (stands for Start Of Authority) to asset it's authority. Since discodns can support multiple authoritative domains, it's up to you to enter this `SOA` record for each domain you use. Here's an example of creating this record for `discodns.net.`.

```shell
curl -L http://127.0.0.1:4001/v2/keys/net/discodns/.SOA -XPUT -d value="ns1.discodns.net\tadmin.discodns.net\t3600\t600\t86400\t10"
{"action":"set","node":{"key":"/net/discodns/.SOA","value":"...","modifiedIndex":11,"createdIndex":11}}
```

Let's break out the value and see what we've got.

```
ns1.discodns.net \t     << - This is the root, master nameserver for this delegated domain
admin.discodns.net \t   << - This is the "admin" email address, note the first segment is actually the user (`admin@discodns.net`)
3600 \t                 << - Time in seconds for any secondary DNS servers to cache the zone (used with `AXFR`)
600 \t                  << - Interval in seconds for any secondary DNS servers to re-try in the event of a failed zone update
86400 \t                << - Expiry time in seconds for any secondary DNS server to drop the zone data (too old)
10                      << - Minimum TTL that applies to all DNS entries in the zone
```

**Note:** If you're familiar with SOA records, you'll probably notice a value missing from above. The "Serial Number" (should be in the 3rd position) is actually filled in automatically by discodns, because it uses the current index of the etcd cluster to describe the current version of the zone.

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
