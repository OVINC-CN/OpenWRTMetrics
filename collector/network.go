package collector

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// network interface metrics collector
type NetworkCollector struct {
	rxBytes   *prometheus.Desc
	txBytes   *prometheus.Desc
	uptime    *prometheus.Desc
	rxPackets *prometheus.Desc
	txPackets *prometheus.Desc
}

// create a new network collector
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		rxBytes: prometheus.NewDesc(
			"openwrt_network_receive_bytes_total",
			"total number of bytes received on network interface",
			[]string{"interface"}, nil,
		),
		txBytes: prometheus.NewDesc(
			"openwrt_network_transmit_bytes_total",
			"total number of bytes transmitted on network interface",
			[]string{"interface"}, nil,
		),
		rxPackets: prometheus.NewDesc(
			"openwrt_network_receive_packets_total",
			"total number of packets received on network interface",
			[]string{"interface"}, nil,
		),
		txPackets: prometheus.NewDesc(
			"openwrt_network_transmit_packets_total",
			"total number of packets transmitted on network interface",
			[]string{"interface"}, nil,
		),
		uptime: prometheus.NewDesc(
			"openwrt_network_uptime_seconds",
			"network interface uptime in seconds",
			[]string{"interface"}, nil,
		),
	}
}

// describe implements prometheus.Collector
func (c *NetworkCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.rxBytes
	ch <- c.txBytes
	ch <- c.rxPackets
	ch <- c.txPackets
	ch <- c.uptime
}

// collect implements prometheus.Collector
func (c *NetworkCollector) Collect(ch chan<- prometheus.Metric) {
	interfaces, err := getNetworkInterfaces()
	if err != nil {
		log.Printf("error collecting network metrics: %v", err)
		return
	}

	for _, iface := range interfaces {
		ch <- prometheus.MustNewConstMetric(
			c.rxBytes,
			prometheus.CounterValue,
			float64(iface.RxBytes),
			iface.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.txBytes,
			prometheus.CounterValue,
			float64(iface.TxBytes),
			iface.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.rxPackets,
			prometheus.CounterValue,
			float64(iface.RxPackets),
			iface.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.txPackets,
			prometheus.CounterValue,
			float64(iface.TxPackets),
			iface.Name,
		)

		// get interface uptime from /sys/class/net/<interface>/statistics/uptime or use system uptime
		uptime := getInterfaceUptime(iface.Name)
		ch <- prometheus.MustNewConstMetric(
			c.uptime,
			prometheus.GaugeValue,
			uptime,
			iface.Name,
		)
	}
}

// networkinterface represents a network interface
type NetworkInterface struct {
	Name      string
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
}

// get network interfaces from /proc/net/dev
func getNetworkInterfaces() ([]NetworkInterface, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var interfaces []NetworkInterface
	scanner := bufio.NewScanner(file)

	// skip first two header lines
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 17 {
			continue
		}

		// interface name is in format "eth0:" or "wlan0:"
		name := strings.TrimSuffix(fields[0], ":")

		// skip loopback interface
		if name == "lo" {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[1], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[2], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[9], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[10], 10, 64)

		interfaces = append(interfaces, NetworkInterface{
			Name:      name,
			RxBytes:   rxBytes,
			TxBytes:   txBytes,
			RxPackets: rxPackets,
			TxPackets: txPackets,
		})
	}

	return interfaces, scanner.Err()
}

// get interface uptime, fallback to system uptime
func getInterfaceUptime(_ string) float64 {
	// try to read system uptime as a proxy
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}

	return uptime
}
