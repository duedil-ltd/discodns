
discodns
======

A DNS resolver that first queries a populated database of names and records, then falls back onto a list of configured nameservers - google DNS by default. This is a great tool to aid development of service discovery systems, without imposing any restrictions on domain structure or architecture.

### Why etcd?

The choice was made as [etcd](http://github.com/coreos/etcd) is a simple, distributable k/v store with dome very useful features that lend themselves well to solving the problem of service discovery. This DNS resolver is not designed to be the single point for discovering services throughout a network, but to make it easier. Services should use etcd to public and watch for changes to records and act accordingly.

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

### Storage

The records are stored in a reverse domain structure, i.e `duedil.com` would equate to the key `com/duedil`. See the examples below;

- `duedil.net. -> A -> [10.1.1.1, 10.1.1.2]`
    - `/net/duedil/.A/foo -> 10.1.1.1`
    - `/net/duedil/.A/bar -> 10.1.1.2`

You'll notice the `.A` folder here on the end of the reverse domain, this signifies to the dns resolver that the values beneath are A records. You can have an infinite number of nested keys within this folder, allowing for some very interesting automation patterns. *Multiple keys within this folder represent multiple records for the same dns entry*.

````shell
; <<>> DiG 9.8.3-P1 <<>> @localhost duedil.net.
; .. truncated ..

;; QUESTION SECTION:
;duedil.net.            IN  A

;; ANSWER SECTION:
duedil.net.     0   IN  A   10.1.1.1
duedil.net.     0   IN  A   10.1.1.2
````

## Notes

Only a select few of record types are supported right now. These are listed here;

- `A` (ipv4)
- `AAAA` (ipv6)
- `TXT`
- `CNAME`
- `NS`
