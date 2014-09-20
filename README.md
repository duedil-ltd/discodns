
discodns
======

[![Build Status](https://travis-ci.org/duedil-ltd/discodns.png?branch=master)](https://travis-ci.org/duedil-ltd/discodns)

An authoritative DNS nameserver that queries an [etcd](http://github.com/coreos/etcd) database of domains and records.

#### Key Features

- Full support for a variety of resource records
    - Both IPv4 (`A`) and IPv6 (`AAAA`) addresses
    - `CNAME` alias records
    - Delegation via `NS` and `SOA` records
    - `SRV` and `PTR` for service discovery and reverse domain lookups
- Multiple resource records of different types per domain (where valid)
- Support for wildcard domains
- Support for TTLs
    - Global default on all records
    - Individual TTL values for individual records
- Runtime and application metrics are captured regularly for monitoring (stdout or graphite)
- Incoming query filters

#### Production Readyness

We've been running discodns in production for several months now, with no issue, though that is not to say it's bug free! If you find any issues, please submit a [bug report](https://github.com/duedil-ltd/discodns/issues/new) or pull request.

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
curl -L http://127.0.0.1:4001/v2/keys/net/discodns/.SOA -XPUT -d value=$'ns1.discodns.net.\tadmin.discodns.net.\t3600\t600\t86400\t10'
{"action":"set","node":{"key":"/net/discodns/.SOA","value":"...","modifiedIndex":11,"createdIndex":11}}
```

Let's break out the value and see what we've got.

```
ns1.discodns.net     << - This is the root, master nameserver for this delegated domain
admin.discodns.net   << - This is the "admin" email address, note the first segment is actually the user (`admin@discodns.net`)
3600                 << - Time in seconds for any secondary DNS servers to cache the zone (used with `AXFR`)
600                  << - Interval in seconds for any secondary DNS servers to re-try in the event of a failed zone update
86400                << - Expiry time in seconds for any secondary DNS server to drop the zone data (too old)
10                   << - Minimum TTL that applies to all DNS entries in the zone
```

These are all tab-separated in the PUT request body. (The `$''` is just a convenience to neatly escape tabs in bash; you could use regular bash strings, with `\u0009` or `%09` for the tab chars, too)

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

## Storage

The record names are used as etcd key prefixes. They are in a reverse domain format, i.e `discodns.net` would equate to the key `net/discodns`. See the examples below;

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

### Record Types

Only a select few of record types are supported right now. These are listed here:

- `A` (ipv4)
- `AAAA` (ipv6)
- `TXT`
- `CNAME`
- `NS`
- `PTR`
- `SRV`

### TTLs (Time To Live)

You can configure discodns with a default TTL (the default default is `300` seconds) using the `--default-ttl` command line option. This means every single DNS resource record returned will have a TTL of the default value, unless otherwise specified on a per-record basis.

To change the TTL of any given record, you can use the `.ttl` suffix. For example, to use a TTL of 60 minutes for the `discodns.net.` A record, the database might look like this...

- `/net/discodns/.A -> 10.1.1.1`
- `/net/discodns/.A.ttl -> 18000`

If you have multiple nested records, you can still use the suffix. In this example, the record identified by `foo` will use the default TTL, and `bar` will use the specified TTL of 60 minutes.

- `/net/discodns/.TXT/foo -> foo`
- `/net/discodns/.TXT/bar -> bar`
- `/net/discodns/.TXT/bar.ttl -> 18000`

### Value storage formats

All records in etcd are, of course, just strings. Most record types only require simple string values with no special considerations, except their natural constraints and types within DNS (valid IP addresses, for example)

In cases where multiple pieces of information are needed for a record, they are separated with **single tab characters**.

These more complex cases are:

#### SOA

Consists of the following tab-delimited fields in order:

- Primary nameserver
- 'Responsible Person' (admin email)
- Refresh
- Retry
- Expire
- Minimum TTL

See the SOA example above for more details

### SRV

Consists of the following tab-delimited fields in order:

- Priority
    - For clients wishing to choose between multiple service instances
    - 16bit unsigned int
- Weight
    - For clients wishing to choose between multiple service instances
    - 16bit unsigned int
- Port
    - The standard port number where the service can be found on the host
    - 16bit unsigned int
- Target
    - a regular domain name for the host where the service can be found
    - _must_ be resolvable to an A/AAAA record.

For more about the Priority and Weight fields, including the algorithm to use when choosing, see [RFC2782](https://www.ietf.org/rfc/rfc2782.txt).

## Metrics

The discodns server will monitor a wide range of runtime and application metrics. By default these metrics are dumped to stderr every 30 seconds, but this can be configured using the `-metrics` argument, set to `0` to disable completely.

You can also use the `-graphite` arguments for shipping metrics to your own Graphite server instead.

## Query Filters

In some situations, it can be useful to restrict the activities of a discodns nameserver to avoid querying etcd for certain domains or record types. For example, your network may not have support for IPv6 and therefore will never be storing any internal `AAAA` records, so it's a waste of effort querying etcd as they're never going to return with values.

This can be achieved with the `--accept` and `--reject` options to discodns. With these options, queries will be tested against the acceptance criteria before hitting etcd, or the internal resolver. This is a very cheap operation, and can drastically improve performance in some cases.

For example, if I **only** want to allow PTR lookups in the `in-addr.arpa.` domain space (for reverse domain queries) I can use the `--accept="in-addr.arpa:PTR"` argument. The nameserver is now going to reject any queries that aren't reverse lookups.

```
--accept="discodns.net:" # Accept any queries within the discodns.net domain
--accept="discodns.net:SRV,PTR" # Accept only PTR and SRV queries within the discodns domain
--reject="discodns.net:AAAA" # Reject any queries within the discodns.net domain that are for IPv6 lookups
```

## Contributions

All contributions are welcome and encouraged! Please feel free to open a pull request no matter how large or small.

The project is [licensed under MIT](https://github.com/duedil-ltd/discodns/blob/master/LICENSE).
