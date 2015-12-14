package main

import (
	"github.com/coreos/go-etcd/etcd"
	"github.com/jessevdk/go-flags"
	"github.com/miekg/dns"
	"github.com/rcrowley/go-metrics"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

var (
	logger    = log.New(os.Stderr, "[discodns] ", log.Ldate|log.Ltime)
	log_debug = false

	// Define all of the command line arguments
	Options struct {
		ListenAddress    string   `short:"l" long:"listen" description:"Listen IP address" default:"0.0.0.0"`
		ListenPort       int      `short:"p" long:"port" description:"Port to listen on" default:"53"`
		EtcdHosts        []string `short:"e" long:"etcd" description:"host:port[,host:port] for etcd hosts" default:"127.0.0.1:4001"`
		Debug            bool     `short:"v" long:"debug" description:"Enable debug logging"`
		MetricsDuration  int      `short:"m" long:"metrics" description:"Dump metrics to stderr every N seconds" default:"30"`
		GraphiteServer   string   `long:"graphite" description:"Graphite server to send metrics to"`
		GraphiteDuration int      `long:"graphite-duration" description:"Duration to periodically send metrics to the graphite server" default:"10"`
		DefaultTtl       uint32   `short:"t" long:"default-ttl" description:"Default TTL to return on records without an explicit TTL" default:"300"`
		Accept           []string `long:"accept" description:"Limit DNS queries to a set of domain:[type,...] pairs"`
		Reject           []string `long:"reject" description:"Limit DNS queries to a set of domain:[type,...] pairs"`
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

	// Create an ETCD client
	etcd := etcd.NewClient(Options.EtcdHosts)
	if !etcd.SyncCluster() {
		logger.Printf("[WARNING] Failed to connect to etcd cluster at launch time")
	}

	// Register the metrics writer
	if len(Options.GraphiteServer) > 0 {
		addr, err := net.ResolveTCPAddr("tcp", Options.GraphiteServer)
		if err != nil {
			logger.Fatalf("Failed to parse graphite server: ", err)
		}

		prefix := "discodns"
		hostname, err := os.Hostname()
		if err != nil {
			logger.Fatalf("Unable to get hostname: ", err)
		}

		prefix = prefix + "." + strings.Replace(hostname, ".", "_", -1)

		go metrics.Graphite(metrics.DefaultRegistry, time.Duration(Options.GraphiteDuration)*time.Second, prefix, addr)
	} else if Options.MetricsDuration > 0 {
		go metrics.Log(metrics.DefaultRegistry, time.Duration(Options.MetricsDuration)*time.Second, logger)

		// Register a bunch of debug metrics
		metrics.RegisterDebugGCStats(metrics.DefaultRegistry)
		metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
		go metrics.CaptureDebugGCStats(metrics.DefaultRegistry, time.Duration(Options.MetricsDuration))
		go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, time.Duration(Options.MetricsDuration))
	} else {
		logger.Printf("Metric logging disabled")
	}

	// Start up the DNS resolver server
	server := &Server{
		addr:       Options.ListenAddress,
		port:       Options.ListenPort,
		etcd:       etcd,
		rTimeout:   time.Duration(5) * time.Second,
		wTimeout:   time.Duration(5) * time.Second,
		defaultTtl: Options.DefaultTtl,
		queryFilterer: &QueryFilterer{acceptFilters: parseFilters(Options.Accept),
			rejectFilters: parseFilters(Options.Reject)}}

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

// parseFilters will convert a string into a Query Filter structure. The accepted
// format for input is [domain]:[type,type,...]. For example...
//
// - "domain:A,AAAA" # Match all A and AAAA queries within `domain`
// - ":TXT" # Matches only TXT queries for any domain
// - "domain:" # Matches any query within `domain`
func parseFilters(filters []string) []QueryFilter {
	parsedFilters := make([]QueryFilter, 0)
	for _, filter := range filters {
		components := strings.Split(filter, ":")
		if len(components) != 2 {
			logger.Printf("Expected only one colon ([domain]:[type,type...])")
			continue
		}

		domain := dns.Fqdn(components[0])
		types := strings.Split(components[1], ",")

		if len(types) == 1 && len(types[0]) == 0 {
			types = make([]string, 0)
		}

		debugMsg("Adding filter with domain '" + domain + "' and types '" + strings.Join(types, ",") + "'")
		parsedFilters = append(parsedFilters, QueryFilter{domain, types})
	}

	return parsedFilters
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
