package collector

import (
	"log"
	"net"

	"github.com/prometheus/client_golang/prometheus"
)

// interface ip collector
type InterfaceIPCollector struct {
	ipInfo *prometheus.Desc
}

// create a new interface ip collector
func NewInterfaceIPCollector() *InterfaceIPCollector {
	return &InterfaceIPCollector{
		ipInfo: prometheus.NewDesc(
			"openwrt_interface_ip_info",
			"ip address information for network interfaces",
			[]string{"interface", "ip", "version", "family"}, nil,
		),
	}
}

// describe implements prometheus.Collector
func (c *InterfaceIPCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.ipInfo
}

// collect implements prometheus.Collector
func (c *InterfaceIPCollector) Collect(ch chan<- prometheus.Metric) {
	ipInfos, err := getInterfaceIPAddresses()
	if err != nil {
		log.Printf("error collecting interface ip metrics: %v", err)
		return
	}

	for _, info := range ipInfos {
		ch <- prometheus.MustNewConstMetric(
			c.ipInfo,
			prometheus.GaugeValue,
			1,
			info.InterfaceName,
			info.IP,
			info.Version,
			info.Family,
		)
	}
}

// interface ip information
type InterfaceIPInfo struct {
	InterfaceName string
	IP            string
	Version       string
	Family        string
}

// get ip addresses for all network interfaces
func getInterfaceIPAddresses() ([]InterfaceIPInfo, error) {
	var ipInfos []InterfaceIPInfo

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {

		// skip loopback interface
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// skip down interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("error getting addresses for interface %s: %v", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			version := ""
			family := ""

			if ip.To4() != nil {

				// ipv4 address
				version = "4"
				if ip.IsPrivate() {
					family = "private"
				} else if ip.IsLoopback() {
					family = "loopback"
				} else if ip.IsLinkLocalUnicast() {
					family = "link-local"
				} else {
					family = "public"
				}
			} else if ip.To16() != nil {

				// ipv6 address
				version = "6"
				if ip.IsPrivate() {
					family = "private"
				} else if ip.IsLoopback() {
					family = "loopback"
				} else if ip.IsLinkLocalUnicast() {
					family = "link-local"
				} else if ip.IsGlobalUnicast() {
					family = "global"
				} else {
					family = "other"
				}
			}

			if version != "" {
				ipInfos = append(ipInfos, InterfaceIPInfo{
					InterfaceName: iface.Name,
					IP:            ip.String(),
					Version:       version,
					Family:        family,
				})
			}
		}
	}

	return ipInfos, nil
}
