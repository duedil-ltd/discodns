package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/jessevdk/go-flags"
    "runtime"
    "os/signal"
    "os"
    "log"
    "time"
)

var (
    logger = log.New(os.Stderr, "[discodns] ", log.Ldate|log.Ltime)
    log_debug = false

    // Define all of the command line arguments
    Options struct {
        ListenAddress   string      `short:"l" long:"listen" description:"Listen IP address" default:"0.0.0.0"`
        ListenPort      int         `short:"p" long:"port" description:"Port to listen on" default:"53"`
        EtcdHosts       []string    `short:"e" long:"etcd" description:"host:port for etcd hosts" default:"127.0.0.1:4001"`
        Nameservers     []string    `short:"n" long:"ns" description:"Upstream nameservers for forwarding"`
        Timeout         string      `short:"t" long:"ns-timeout" description:"Default forwarding timeout" default:"1s"`
        Domain          []string    `short:"d" long:"domain" description:"Domain for this server to be authoritative over"`
        Debug           bool        `short:"v" long:"debug" description:"Enable debug logging"`
    }
)

func main() {

    _, err := flags.ParseArgs(&Options, os.Args[1:])
    if err != nil {
        os.Exit(1)
    }

    if Options.Debug {
        log_debug = true
        debugMsg("Debug mode enabled")
    }

    // Parse the timeout string
    nsTimeout, err := time.ParseDuration(Options.Timeout)
    if err != nil {
        logger.Fatalf("Failed to parse duration '%s'", Options.Timeout)
    }

    if len(Options.Nameservers) == 0 {
        logger.Fatalf("Upstream nameservers are required with -n")
    }

    // Create an ETCD client
    etcd := etcd.NewClient(Options.EtcdHosts)
    if !etcd.SyncCluster() {
        logger.Printf("[WARNING] Failed to connect to etcd cluster at launch time")
    }

    // Start up the DNS resolver server
    server := &Server{
        addr: Options.ListenAddress,
        port: Options.ListenPort,
        etcd: etcd,
        rTimeout: nsTimeout,
        wTimeout: nsTimeout,
        domains: Options.Domain,
        ns: Options.Nameservers}

    server.Run()

    logger.Printf("Listening on %s:%d\n", Options.ListenAddress, Options.ListenPort)

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
