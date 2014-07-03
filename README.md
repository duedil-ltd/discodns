
discodns
======

![](https://travis-ci.org/duedil-ltd/discodns.png)

An authoritative DNS nameserver that queries an [etcd](http://github.com/coreos/etcd) database of domains and records.

#### Key Features

- Full support for a variety of resource records
    - Both IPv4 (`A`) and IPv6 (`AAAA`) addresses
    - `CNAME` alias records
    - Delegation via `NS` and `SOA` records
    - `SRV` and `PTR` for service discovery and reverse domain lookups
- Multiple resource records of different types per domain (where valid)
- Runtime and application metrics are captured regularly for monitoring (stdout or grahite)
- Support for wildcard domains

#### Coming Soon

- Support for configurable TTLs on a per-record basis (currently everything has a TTL of 0)
- Support for zone transfers (`AXFR`), though enabling these would be on a short-lived basis
- Better error handling ([see #10](https://github.com/duedil-ltd/discodns/issues/10))

#### Production Readyness

Can I use this in production? TLDR; Yes, with caution.

We've been running discodns in some production environments with little issue, though that is not to say it's bug free! If you find any issues, please submit a [bug report](https://github.com/duedil-ltd/discodns/issues/new) or pull request.

### Why did we build discodns?

When building infrastructure of sufficient complexity -- especially elastic infrastructure -- we've found it's really valuable to have a fast and flexible system for service **identity** and discovery. Crucially, it has to support different naming conventions and work with a wide variety of platform tooling and service software. DNS has proved itself to be capable in that role for over 25 years.

Since discodns is not a recursive resolver, nor does it implement it's own cache, you should front queries with a forwarder ([BIND](http://www.isc.org/downloads/bind/), for example) as seen in the diagram below.

             +-----------+   +---------+
             |           |   |         |
             |  Servers  |   |  Users  |
             |           |   |         |
             +------+----+   +----+----+
                    |             |
             +------v-------------v----+
             |                         |
             |    Forwarders (BIND)    |
             |                         |
             +----+---+---------+------+
                  |   |         |
    +----------+  |   |         |
    |          |  |   |         |
    | discodns <--+   |         |
    |          |      |         |
    +----------+      |         |    +----------------+
                      |         |    |                |
            +---------v---+     +---->   Intertubes   |
            |             |          |                |
            |  Something  |          +----------------+
            |    Else     |
            |             |
            +-------------+

This "pluggable" DNS architecture allows us to mix a variety of tools to achieve a very flexible global discovery system throughout.

### Why etcd?

We chose [etcd](http://github.com/coreos/etcd) as it's a simple and distributed k/v store. It's also commonly used (and designed) for cluster management, so can behave as the canonical  point for discovering services throughout a network. Services can utilize the same etcd cluster to both publish and subscribe to changes in the domain name system, as well as other orchestration needs they may have.

Why not ZooKeeper? The etcd API is much simpler for users to use, and it uses [RAFT](http://raftconsensus.github.io/) instead of [Paxos](http://en.wikipedia.org/wiki/Paxos_(computer_science)), which contributes to it being a simpler to understand and easier to manage.

Another attractive quality about etcd is the ability to continue serving (albeit stale) read queries even when a consensus cannot be reached, allowing the cluster to enter a semi-failed state where it cannot accept writes, but it will serve reads. This kind of graceful service degradation is very useful for a read-heavy system, such as DNS.

## Getting Started

The discodns project is written in Go, and uses an extensive library ([miekg/dns](https://github.com/miekg/dns)) to provide the actual implementation of the DNS protocol.

You'll need to compile from source, though a Makefile is provided to make this easier. Before starting, you'll need to ensure you have [Go](http://golang.org/) (1.2+) installed.

### Building

It's simple enough to compile from source...

````shell
cd discodns
make
````

### Running

It's as simple as launching the binary to start a DNS server listening on port 53 (tcp+udp) and accepting requests. You need to ensure you also have an etcd cluster up and running, [which you can read about here](https://github.com/coreos/etcd#getting-started).

**Note:** You can enable verbose logging using the `-v` argument

**Note:** Since port `53` is a privileged port, you'll need to run discodns as root. You should not do this in production.

````shell
cd discodns/build/
sudo ./bin/discodns --etcd=127.0.0.1:4001
````

### Try it out

It's incredibly easy to see your own domains come to life, simply insert a key for your record into etcd and then you're ready to go! Here we'll insert a custom `A` record for `discodns.net` pointing to `10.1.1.1`.

````shell
curl -L http://127.0.0.1:4001/v2/keys/net/discodns/.A -XPUT -d value="10.1.1.1"
{"action":"set","node":{"key":"/net/discodns/.A","value":"10.1.1.1","modifiedIndex":11,"createdIndex":11}}
````

````shell
$ @localhost discodns.net.
; <<>> DiG 9.8.3-P1 <<>> @localhost discodns.net.
; .. truncated ..

;; QUESTION SECTION:
;discodns.net.            IN  A

;; ANSWER SECTION:
discodns.net.     0   IN  A   10.1.1.1
````

### Authority

If you're not familiar with the DNS specification, to behave *correctly* as an authoritative nameserver each domain needs to have its own `SOA` (stands for Start Of Authority) and `NS` records to assert its authority. Since discodns can support multiple authoritative domains, it's up to you to enter this `SOA` record for each domain you use. Here's an example of creating this record for `discodns.net.`.

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

**Note:** If you're familiar with SOA records, you'll probably notice a value missing from above. The "Serial Number" (should be in the 3rd position) is actually filled in automatically by discodns, because it uses the current index of the etcd cluster to describe the current version of the zone. (TODO).

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

The records are stored in a reverse domain format, i.e `discodns.net` would equate to the key `net/discodns`. See the examples below;

- `discodns.net. -> A record -> [10.1.1.1, 10.1.1.2]`
    - `/net/discodns/.A/foo -> 10.1.1.1`
    - `/net/discodns/.A/bar -> 10.1.1.2`

You'll notice the `.A` folder here on the end of the reverse domain, this signifies to the dns resolver that the values beneath are A records. You can have an infinite number of nested keys within this folder, allowing for some very interesting automation patterns. *Multiple keys within this folder represent multiple records for the same dns entry*. If you want to enforce only one value exists for a record type (CNAME for example) you can use a single key instead of a directory (`/net/discodns/.CNAME -> foo.net`).

````shell
$ dig @localhost discodns.net.
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

Only a select few of record types are supported right now. These are listed here:

- `A` (ipv4)
- `AAAA` (ipv6)
- `TXT`
- `CNAME`
- `NS`
- `PTR`
- `SRV`

## Contributions

All contributions are welcome and encouraged! Please feel free to open a pull request no matter how large or small.

The project is [licensed under MIT](https://github.com/duedil-ltd/discodns/blob/master/LICENSE).
