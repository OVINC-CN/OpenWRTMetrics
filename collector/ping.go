package collector

import (
	"log"
	"net"
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
	Targets  []PingTarget
	Count    int
	Interval time.Duration
	Timeout  time.Duration
}

type IPType string

const (
	IPTypeIPv4 IPType = "IPv4"
	IPTypeIPv6 IPType = "IPv6"
)

// ping target with IP version
type PingTarget struct {
	Host   string
	IPType IPType
}

// create a new ping collector
func NewPingCollector() *PingCollector {
	config := loadPingConfig()

	labels := []string{"target", "ip", "ip_type"}

	return &PingCollector{
		latencyMs: prometheus.NewDesc(
			"openwrt_ping_latency_ms",
			"ping latency in milliseconds",
			labels, nil,
		),
		packetLoss: prometheus.NewDesc(
			"openwrt_ping_packet_loss_percent",
			"ping packet loss percentage",
			labels, nil,
		),
		minLatencyMs: prometheus.NewDesc(
			"openwrt_ping_min_latency_ms",
			"minimum ping latency in milliseconds",
			labels, nil,
		),
		maxLatencyMs: prometheus.NewDesc(
			"openwrt_ping_max_latency_ms",
			"maximum ping latency in milliseconds",
			labels, nil,
		),
		avgLatencyMs: prometheus.NewDesc(
			"openwrt_ping_avg_latency_ms",
			"average ping latency in milliseconds",
			labels, nil,
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
			log.Printf("error pinging target %s: %v", target.Host, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.avgLatencyMs,
			prometheus.GaugeValue,
			result.AvgLatencyMs,
			target.Host, result.IP, result.IPType,
		)

		ch <- prometheus.MustNewConstMetric(
			c.minLatencyMs,
			prometheus.GaugeValue,
			result.MinLatencyMs,
			target.Host, result.IP, result.IPType,
		)

		ch <- prometheus.MustNewConstMetric(
			c.maxLatencyMs,
			prometheus.GaugeValue,
			result.MaxLatencyMs,
			target.Host, result.IP, result.IPType,
		)

		ch <- prometheus.MustNewConstMetric(
			c.packetLoss,
			prometheus.GaugeValue,
			result.PacketLoss,
			target.Host, result.IP, result.IPType,
		)

	}
}

// ping result
type PingResult struct {
	MinLatencyMs float64
	MaxLatencyMs float64
	AvgLatencyMs float64
	PacketLoss   float64
	IP           string
	IPType       string
}

// load ping configuration from environment variables
func loadPingConfig() *PingConfig {
	config := &PingConfig{
		Count:    10,
		Interval: 10 * time.Millisecond,
		Timeout:  3 * time.Second,
	}

	// ping_targets: comma-separated list of IPv4 targets
	targetsEnv := os.Getenv("PING_TARGETS")
	if targetsEnv != "" {
		targets := strings.Split(targetsEnv, ",")
		for _, target := range targets {
			target = strings.TrimSpace(target)
			if target != "" {
				config.Targets = append(config.Targets, PingTarget{Host: target, IPType: IPTypeIPv4})
			}
		}
	}

	// ping_targets_v6: comma-separated list of IPv6 targets
	targetsV6Env := os.Getenv("PING_TARGETS_V6")
	if targetsV6Env != "" {
		targets := strings.Split(targetsV6Env, ",")
		for _, target := range targets {
			target = strings.TrimSpace(target)
			if target != "" {
				config.Targets = append(config.Targets, PingTarget{Host: target, IPType: IPTypeIPv6})
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
func pingTarget(target PingTarget, config *PingConfig) (*PingResult, error) {

	// resolve IP address first to determine IP type
	var resolvedIP net.IP

	// lookup IPs
	ips, err := net.LookupIP(target.Host)
	if err != nil {
		return nil, err
	}

	switch target.IPType {
	case IPTypeIPv4:
		for _, ip := range ips {
			if ip.To4() != nil {
				resolvedIP = ip
				break
			}
		}
	case IPTypeIPv6:
		for _, ip := range ips {
			if ip.To4() == nil && ip.To16() != nil {
				resolvedIP = ip
				break
			}
		}
	default:
		return nil, &net.AddrError{Err: "unknown IP type", Addr: target.Host}
	}
	if resolvedIP == nil {
		return nil, &net.AddrError{Err: "no IPv6 address found", Addr: target.Host}
	}

	// create pinger with resolved IP
	pinger, err := probing.NewPinger(resolvedIP.String())
	if err != nil {
		return nil, err
	}

	// set privileged mode to true to use icmp (requires root)
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
		IP:           resolvedIP.String(),
		IPType:       string(target.IPType),
	}

	return result, nil
}
