package collector

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/prometheus/client_golang/prometheus"
)

// ping collector
type PingCollector struct {
	latencyMs    *prometheus.Desc
	packetLoss   *prometheus.Desc
	minLatencyMs *prometheus.Desc
	maxLatencyMs *prometheus.Desc
	avgLatencyMs *prometheus.Desc
	config       *PingConfig
}

// ping configuration
type PingConfig struct {
	Targets  []string
	Count    int
	Interval time.Duration
	Timeout  time.Duration
}

// create a new ping collector
func NewPingCollector() *PingCollector {
	config := loadPingConfig()

	return &PingCollector{
		latencyMs: prometheus.NewDesc(
			"openwrt_ping_latency_ms",
			"ping latency in milliseconds",
			[]string{"target"}, nil,
		),
		packetLoss: prometheus.NewDesc(
			"openwrt_ping_packet_loss_percent",
			"ping packet loss percentage",
			[]string{"target"}, nil,
		),
		minLatencyMs: prometheus.NewDesc(
			"openwrt_ping_min_latency_ms",
			"minimum ping latency in milliseconds",
			[]string{"target"}, nil,
		),
		maxLatencyMs: prometheus.NewDesc(
			"openwrt_ping_max_latency_ms",
			"maximum ping latency in milliseconds",
			[]string{"target"}, nil,
		),
		avgLatencyMs: prometheus.NewDesc(
			"openwrt_ping_avg_latency_ms",
			"average ping latency in milliseconds",
			[]string{"target"}, nil,
		),
		config: config,
	}
}

// describe implements prometheus.Collector
func (c *PingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.latencyMs
	ch <- c.packetLoss
	ch <- c.minLatencyMs
	ch <- c.maxLatencyMs
	ch <- c.avgLatencyMs
}

// collect implements prometheus.Collector
func (c *PingCollector) Collect(ch chan<- prometheus.Metric) {
	if len(c.config.Targets) == 0 {
		return
	}

	for _, target := range c.config.Targets {
		result, err := pingTarget(target, c.config)
		if err != nil {
			log.Printf("error pinging target %s: %v", target, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.avgLatencyMs,
			prometheus.GaugeValue,
			result.AvgLatencyMs,
			target,
		)

		ch <- prometheus.MustNewConstMetric(
			c.minLatencyMs,
			prometheus.GaugeValue,
			result.MinLatencyMs,
			target,
		)

		ch <- prometheus.MustNewConstMetric(
			c.maxLatencyMs,
			prometheus.GaugeValue,
			result.MaxLatencyMs,
			target,
		)

		ch <- prometheus.MustNewConstMetric(
			c.packetLoss,
			prometheus.GaugeValue,
			result.PacketLoss,
			target,
		)

	}
}

// ping result
type PingResult struct {
	MinLatencyMs float64
	MaxLatencyMs float64
	AvgLatencyMs float64
	PacketLoss   float64
}

// load ping configuration from environment variables
func loadPingConfig() *PingConfig {
	config := &PingConfig{
		Count:    10,
		Interval: 10 * time.Millisecond,
		Timeout:  3 * time.Second,
	}

	// ping_targets: comma-separated list of targets
	targetsEnv := os.Getenv("PING_TARGETS")
	if targetsEnv != "" {
		targets := strings.Split(targetsEnv, ",")
		for _, target := range targets {
			target = strings.TrimSpace(target)
			if target != "" {
				config.Targets = append(config.Targets, target)
			}
		}
	}

	// ping_count: number of ping packets to send
	if countEnv := os.Getenv("PING_COUNT"); countEnv != "" {
		if count, err := strconv.Atoi(countEnv); err == nil && count > 0 {
			config.Count = count
		}
	}

	// ping_interval: interval between ping packets in seconds
	if intervalEnv := os.Getenv("PING_INTERVAL"); intervalEnv != "" {
		if interval, err := time.ParseDuration(intervalEnv); err == nil && interval > 0 {
			config.Interval = interval
		}
	}

	// ping_timeout: ping timeout in seconds
	if timeoutEnv := os.Getenv("PING_TIMEOUT"); timeoutEnv != "" {
		if timeout, err := time.ParseDuration(timeoutEnv); err == nil && timeout > 0 {
			config.Timeout = timeout
		}
	}

	return config
}

// ping a target and return the result
func pingTarget(target string, config *PingConfig) (*PingResult, error) {

	// create pinger
	pinger, err := probing.NewPinger(target)
	if err != nil {
		return nil, err
	}

	// set privileged mode to false to use udp instead of icmp (no root required)
	pinger.SetPrivileged(true)

	// configure ping parameters
	pinger.Count = config.Count
	pinger.Interval = config.Interval
	pinger.Timeout = config.Timeout

	// run ping
	err = pinger.Run()
	if err != nil {
		return nil, err
	}

	// get statistics
	stats := pinger.Statistics()

	result := &PingResult{
		PacketLoss:   stats.PacketLoss,
		MinLatencyMs: float64(stats.MinRtt.Microseconds()) / 1000.0,
		MaxLatencyMs: float64(stats.MaxRtt.Microseconds()) / 1000.0,
		AvgLatencyMs: float64(stats.AvgRtt.Microseconds()) / 1000.0,
	}

	return result, nil
}
