package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "runtime"
    "os/signal"
    "os"
    "strings"
    "log"
    "flag"
    "time"
)

var (
    logger = log.New(os.Stderr, "[discodns] ", log.Ldate|log.Ltime)
    log_debug = false
)

func main() {

    var addr = flag.String("listen", "0.0.0.0", "Listen IP address")
    var port = flag.Int("port", 53, "Port to listen on")
    var hosts = flag.String("etcd", "0.0.0.0:4001", "List of etcd hosts (comma separated)")
    var nameservers = flag.String("ns", "8.8.8.8,8.8.4.4", "Fallback nameservers (comma separated)")
    var timeout = flag.String("ns-timeout", "1s", "Default nameserver timeout")
    var domain = flag.String("domain", "discodns.local", "Constrain discodns to a domain")
    var authority = flag.String("authority", "dns.discodns.local", "Authoritative DNS server hostname")
    var debug = flag.Bool("debug", false, "Enable debug logging")

    flag.Parse()

    if *debug {
        log_debug = true
        debugMsg("Debug mode enabled")
    }

    // Parse the list of nameservers
    ns := strings.Split(*nameservers, ",")

    // Parse the timeout string
    nsTimeout, err := time.ParseDuration(*timeout)
    if err != nil {
        logger.Fatalf("Failed to parse duration '%s'", timeout)
    }

    // Connect to ETCD (wait for a connection)
    etcd := etcd.NewClient(strings.Split(*hosts, ","))

    if !etcd.SyncCluster() {
        logger.Printf("[WARNING] Failed to connect to etcd cluster at launch time")
    }

    // Start up the DNS resolver server
    server := &Server{
        addr: *addr,
        port: *port,
        etcd: etcd,
        rTimeout: nsTimeout,
        wTimeout: nsTimeout,
        domain: dns.Fqdn(*domain),
        authority: dns.Fqdn(*authority),
        ns: ns}

    server.Run()

    logger.Printf("Listening on %s:%d\n", *addr, *port)

    sig := make(chan os.Signal)
    signal.Notify(sig, os.Interrupt)

forever:
    for {
        select {
        case <-sig:
            logger.Printf("Bye bye :(\n")
            break forever
        }
    }
}

func debugMsg(v ...interface{}) {
    if log_debug {
        vars := []interface{}{"[", runtime.NumGoroutine(), "]"}
        vars = append(vars, v...)

        logger.Println(vars...)
    }
}

func init() {
    runtime.GOMAXPROCS(runtime.NumCPU())
}
