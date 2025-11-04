package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/ovinc/openwrt-metrics/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	listenAddress = flag.String("listen-address", ":9101", "address to listen on for metrics")
	metricsPath   = flag.String("metrics-path", "/metrics", "path under which to expose metrics")
)

const homePage = `<html>
<head><title>OpenWRT Exporter</title></head>
<body>
<h1>OpenWRT Exporter</h1>
<p><a href="%s">Metrics</a></p>
</body>
</html>`

func main() {
	flag.Parse()

	log.Printf("starting openwrt exporter on %s", *listenAddress)

	// create custom registry
	registry := prometheus.NewRegistry()

	// register collectors
	registry.MustRegister(collector.NewNetworkCollector())
	registry.MustRegister(collector.NewDeviceCollector())

	// setup http handler
	http.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(homePage, *metricsPath)))
	})

	log.Printf("listening on %s, exposing metrics on %s", *listenAddress, *metricsPath)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
