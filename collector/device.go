package collector

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// connected device metrics collector
type DeviceCollector struct {
	deviceInfo        *prometheus.Desc
	deviceOnlineTime  *prometheus.Desc
	deviceLeaseRemain *prometheus.Desc
}

// create a new device collector
func NewDeviceCollector() *DeviceCollector {
	return &DeviceCollector{
		deviceInfo: prometheus.NewDesc(
			"openwrt_device_info",
			"information about connected devices",
			[]string{"hostname", "ip", "mac"}, nil,
		),
		deviceOnlineTime: prometheus.NewDesc(
			"openwrt_device_online_seconds",
			"device online time in seconds",
			[]string{"hostname", "ip", "mac"}, nil,
		),
		deviceLeaseRemain: prometheus.NewDesc(
			"openwrt_device_dhcp_lease_remaining_seconds",
			"dhcp lease remaining time in seconds",
			[]string{"hostname", "ip", "mac"}, nil,
		),
	}
}

// describe implements prometheus.Collector
func (c *DeviceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.deviceInfo
	ch <- c.deviceOnlineTime
	ch <- c.deviceLeaseRemain
}

// collect implements prometheus.Collector
func (c *DeviceCollector) Collect(ch chan<- prometheus.Metric) {
	devices, err := getConnectedDevices()
	if err != nil {
		log.Printf("error collecting device metrics: %v", err)
		return
	}

	for _, device := range devices {
		// device info as a constant metric with value 1
		ch <- prometheus.MustNewConstMetric(
			c.deviceInfo,
			prometheus.GaugeValue,
			1,
			device.Hostname,
			device.IP,
			device.MAC,
		)

		// online time if available
		if device.OnlineTime > 0 {
			ch <- prometheus.MustNewConstMetric(
				c.deviceOnlineTime,
				prometheus.GaugeValue,
				device.OnlineTime,
				device.Hostname,
				device.IP,
				device.MAC,
			)
		}

		// dhcp lease remaining time
		if device.LeaseRemain > 0 {
			ch <- prometheus.MustNewConstMetric(
				c.deviceLeaseRemain,
				prometheus.GaugeValue,
				device.LeaseRemain,
				device.Hostname,
				device.IP,
				device.MAC,
			)
		}
	}
}

// connected device information
type ConnectedDevice struct {
	Hostname    string
	IP          string
	MAC         string
	OnlineTime  float64
	LeaseRemain float64
}

// get connected devices from dhcp leases and arp table
func getConnectedDevices() ([]ConnectedDevice, error) {

	// use composite key (mac+ip) to support both ipv4 and ipv6
	devices := make(map[string]*ConnectedDevice)

	// read dhcp leases from /tmp/dhcp.leases or /var/dhcp.leases
	dhcpDevices, err := parseDHCPLeases()
	if err != nil {
		log.Printf("warning: failed to read dhcp leases: %v", err)
	} else {
		for _, d := range dhcpDevices {
			key := d.MAC + "|" + d.IP
			devices[key] = d
		}
	}

	// read arp table to get additional connected devices
	arpDevices, err := parseARPTable()
	if err != nil {
		log.Printf("warning: failed to read arp table: %v", err)
	} else {
		for _, d := range arpDevices {
			key := d.MAC + "|" + d.IP
			if _, ok := devices[key]; !ok {
				devices[key] = d
			}
		}
	}

	// convert map to slice
	var result []ConnectedDevice
	for _, device := range devices {

		// ensure we have at least ip or mac
		if device.IP != "" || device.MAC != "" {
			result = append(result, *device)
		}
	}

	return result, nil
}

// parse dhcp leases file
func parseDHCPLeases() ([]*ConnectedDevice, error) {
	// try common locations for dhcp leases file
	leasePaths := []string{
		"/tmp/dhcp.leases",
		"/var/lib/misc/dnsmasq.leases",
		"/tmp/dnsmasq.leases",
	}

	var file *os.File
	var err error

	for _, path := range leasePaths {
		file, err = os.Open(path)
		if err == nil {
			defer func() { _ = file.Close() }()
			break
		}
	}

	if file == nil {
		return nil, err
	}

	var devices []*ConnectedDevice
	scanner := bufio.NewScanner(file)
	now := time.Now().Unix()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// dnsmasq lease format: <expiry_time> <mac> <ip> <hostname> <client_id>
		if len(fields) >= 4 {
			expiryTime, _ := strconv.ParseInt(fields[0], 10, 64)
			mac := fields[1]
			ip := fields[2]
			hostname := fields[3]

			if hostname == "*" {
				hostname = ""
			}

			leaseRemain := float64(0)
			if expiryTime > now {
				leaseRemain = float64(expiryTime - now)
			}

			devices = append(devices, &ConnectedDevice{
				Hostname:    hostname,
				IP:          ip,
				MAC:         mac,
				LeaseRemain: leaseRemain,
				OnlineTime:  0, // not available from dhcp leases
			})
		}
	}

	return devices, scanner.Err()
}

// parse arp table to get connected devices
func parseARPTable() ([]*ConnectedDevice, error) {
	// try to use 'ip neigh' command first (more modern)
	output, err := exec.Command("ip", "neigh", "show").Output()
	if err == nil {
		return parseIPNeigh(string(output))
	}

	// fallback to /proc/net/arp
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var devices []*ConnectedDevice
	scanner := bufio.NewScanner(file)

	// skip header line
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// arp format: IP HWtype Flags HWaddress Mask Device
		if len(fields) >= 4 {
			ip := fields[0]
			mac := fields[3]

			// skip incomplete entries
			if mac == "00:00:00:00:00:00" || fields[2] == "0x0" {
				continue
			}

			devices = append(devices, &ConnectedDevice{
				Hostname:    "",
				IP:          ip,
				MAC:         mac,
				LeaseRemain: 0,
				OnlineTime:  0,
			})
		}
	}

	return devices, scanner.Err()
}

// parse output of 'ip neigh show' command
func parseIPNeigh(output string) ([]*ConnectedDevice, error) {
	var devices []*ConnectedDevice
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// format: <ip> dev <interface> lladdr <mac> <state>
		if len(fields) >= 5 {
			ip := fields[0]
			mac := ""

			// find lladdr (link layer address)
			for i, field := range fields {
				if field == "lladdr" && i+1 < len(fields) {
					mac = fields[i+1]
					break
				}
			}

			if mac != "" {
				devices = append(devices, &ConnectedDevice{
					Hostname:    "",
					IP:          ip,
					MAC:         mac,
					LeaseRemain: 0,
					OnlineTime:  0,
				})
			}
		}
	}

	return devices, scanner.Err()
}
