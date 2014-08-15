package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/jessevdk/go-flags"
    "github.com/nu7hatch/gouuid"
    "log"
    "os"
)

var (
    logger = log.New(os.Stderr, "[discodns-bench] ", log.Ldate|log.Ltime)
    Options struct {
        EtcdHosts       []string    `short:"e" long:"etcd" description:"host:port[,host:port] for etcd hosts" default:"127.0.0.1:4001"`
        NDomains        int         `short:"n" long:"domains" description:"Number of domains to generate" default:"1000"`
        NRecords        int         `short:"r" long:"records" description:"Number of records to generate per domain" default:"5"`
    }
)

func main() {

    _, err := flags.ParseArgs(&Options, os.Args[1:])
    if err != nil {
        logger.Printf(err.Error())
        os.Exit(1)
    }

    // Connect to etcd
    etcd := etcd.NewClient(Options.EtcdHosts)
    if !etcd.SyncCluster() {
        logger.Printf("[ERROR] Failed to connect to etcd cluster")
        os.Exit(1)
    }

    // Check the /benchmark directory doesn't exist
    benchmark_key, err := etcd.Get("benchmark", false, false)
    if benchmark_key != nil {
        logger.Printf("[ERROR] You must run the benchmark against a fresh etcd cluster")
        os.Exit(1)
    }

    etcdPath := "/benchmark"
    logger.Printf("Generating fixture records...")

    domains := make([]string, Options.NDomains)

    // Create all the fixture domains
    for i := 0; i < Options.NDomains; i++ {

        // Generate a unique domain
        domain_uuid, err := uuid.NewV4()
        if err != nil {
            logger.Printf("Error generating UUID: " + err.Error())
            continue
        }

        domainPath := etcdPath + "/net/disco/benchmark/" + domain_uuid.String()
        domain := domain_uuid.String() + ".benchmark.disco.net"

        logger.Printf("Creating domain " + domain + " at " + domainPath)
        domains = append(domains, domain)

        for r := 0; r < Options.NRecords; r++ {
            etcd.CreateInOrder(domainPath + "/.A", "1.1.1.1", 0)    
        }
    }
}
