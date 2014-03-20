
etcdns
======

A DNS resolver that first queries a populated database of names and records, then falls back onto a list of configured nameservers - google DNS by default. This is a great tool for service discovery systems with.

## Getting Started

### Building

The etcdns project is written in Go, and uses an extensive library ([miekg/dns](https://github.com/miekg/dns)) to provide the actual implementation of the DNS protocol.

````shell
cd etcdns
go get  # Ignore the error about installing etcdns
make
````

### Running

It's as simple as launching the binary to start a DNS server listening on port 53 (tcp+udp) and accepting requests.

**Note:** You need to have an etcd cluster already running.

````shell
cd etcdns/build/
sudo ./bin/etcdns
````

### Try it out

It's incredibly easy to see your own domains come to life, simply insert a key for your record into etcd and then you're ready to go! Here we'll insert a custom `A` record for `etcdns.net` pointing to `10.1.1.1`.

````shell
curl -L http://127.0.0.1:4001/v2/keys/net/etcdns/.A -XPUT -d value="10.1.1.1"
{"action":"set","node":{"key":"/net/etcdns/.A","value":"10.1.1.1","modifiedIndex":11,"createdIndex":11}}
````

````shell
; <<>> DiG 9.8.3-P1 <<>> @localhost etcdns.net.
; .. truncated ..

;; QUESTION SECTION:
;etcdns.net.            IN  A

;; ANSWER SECTION:
etcdns.net.     0   IN  A   10.1.1.1
````

## Notes

Only a select few of record types are supported right now. These are listed here;

- `A` (ipv4)
- `AAAA` (ipv6)
- `TXT`
- `CNAME`
