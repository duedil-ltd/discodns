
discodns
======

An authoritative DNS *nameserver* that queries an [etcd](http://github.com/coreos/etcd) database of domains and records.

#### Key Features

- Full support for both IPv4 (`A`) and IPv6 (`AAAA`) addresses
- Full support for `CNAME` alias records
- Full support for both `NS` and `SOA` records for delegation to other nameservers
- Multiple resource records of different types per domain (where valid)
- Runtime and application metrics are captured regularly for monitoring (stdout or grahite)
- Support for wildcard domains

#### Coming Soon

- Support for SRV records
- Support for configurable TTLs on a per-record basis
- Support for zone transfers (`AXFR`), though enabling these would be on a short-lived basis

### Why is this useful?

We built discodns to be the backbone of our internal DNS infrastructure, providing us with a robust and distributed nameserver system to use. It allows us to register domains in any format, with a variety of record types, without imposing restrictions on how hosts or services are named (unlike other DNS service discovery systems). It was also important to us for it to be very easy and fast to add or remove DNS records, so a file-based solution wasn't a simple option.

### Why etcd?

The choice was made as [etcd](http://github.com/coreos/etcd) is a simple, distributable k/v store with some very useful features that lend themselves well to solving the problem of service discovery. This DNS resolver is not designed to be the single point for discovering services throughout a network, but to make it easier. Services can utilize the same etcd cluster to both publish and subscribe to changes in the domain name system.

We chose etcd as an alternative to ZooKeeper for a few reasons. It's API is much simpler for users to use, and it uses [RAFT](http://raftconsensus.github.io/) instead of [Paxos](http://en.wikipedia.org/wiki/Paxos_(computer_science)) (which contributed to it being a simpler to understand and easier to manage system). Also unlike Zookeeper, if a majority of nodes in the cluster enter a failed state, remaining nodes are still able to serve read queries. This is very useful, as it allows the DNS cluster to enter a semi-failed state without being down completely (accepting reads but no writes).

## Getting Started

### Building

The discodns project is written in Go, and uses an extensive library ([miekg/dns](https://github.com/miekg/dns)) to provide the actual implementation of the DNS protocol.

````shell
cd discodns
make
````

### Running

It's as simple as launching the binary to start a DNS server listening on port 53 (tcp+udp) and accepting requests.

**Note:** You can enable verbose logging using the `-v` argument

````shell
cd discodns/build/
sudo ./bin/discodns
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

If you're not familiar with the DNS specification, to behave correctly as an authoritative nameserver each domain needs to have it's own `SOA` record (stands for Start Of Authority) to assert it's authority, as well as a set of `NS` records (most likely a set of other discodns servers). Since discodns can support multiple authoritative domains, it's up to you to enter this `SOA` record for each domain you use. Here's an example of creating this record for `discodns.net.`.

#### SOA

```shell
curl -L http://127.0.0.1:4001/v2/keys/net/discodns/.SOA -XPUT -d value="ns1.discodns.net.\tadmin.discodns.net.\t3600\t600\t86400\t10"
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

#### NS

Let's add the two NS records we need for our DNS cluster.

```
curl -L http://127.0.0.1:4001/v2/keys/net/discodns/.NS/ns1 -XPUT -d value=ns1.discodns.net.
{"action":"set","node":{"key":"/net/discodns/.NS/ns1","value":"...","modifiedIndex":12,"createdIndex":12}}
```

```
curl -L http://127.0.0.1:4001/v2/keys/net/discodns/.NS/ns2 -XPUT -d value=ns2.discodns.net.
{"action":"set","node":{"key":"/net/discodns/.NS/ns2","value":"...","modifiedIndex":13,"createdIndex":13}}
```

**Don't forget to ensure you also add `A` records for the `ns{1,2}.discodns.net` domains to ensure they can resolve to IPs.**

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

### Metrics

The discodns server will monitor a wide range of runtime and application metrics. By default these metrics are dumped to stderr every 30 seconds, but this can be configured using the `-metrics` argument, set to `0` to disable completely.

You can also use the `-graphite` arguments for shipping metrics to your own Graphite server instead.

## Notes

Only a select few of record types are supported right now. These are listed here;

- `A` (ipv4)
- `AAAA` (ipv6)
- `TXT`
- `CNAME`
- `NS`
