package collector

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// UPnP port mapping metrics collector
type UPnPCollector struct {
	upnpInfo         *prometheus.Desc
	upnpLeaseSeconds *prometheus.Desc
	upnpMappingCount *prometheus.Desc
}

// create a new UPnP collector
func NewUPnPCollector() *UPnPCollector {
	return &UPnPCollector{
		upnpInfo: prometheus.NewDesc(
			"openwrt_upnp_mapping_info",
			"information about UPnP port mappings",
			[]string{"protocol", "external_port", "internal_ip", "internal_port", "description"}, nil,
		),
		upnpLeaseSeconds: prometheus.NewDesc(
			"openwrt_upnp_mapping_lease_seconds",
			"UPnP port mapping lease duration in seconds (0 means permanent)",
			[]string{"protocol", "external_port", "internal_ip", "internal_port", "description"}, nil,
		),
		upnpMappingCount: prometheus.NewDesc(
			"openwrt_upnp_mapping_count",
			"total number of active UPnP port mappings",
			nil, nil,
		),
	}
}

// describe implements prometheus.Collector
func (c *UPnPCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upnpInfo
	ch <- c.upnpLeaseSeconds
	ch <- c.upnpMappingCount
}

// collect implements prometheus.Collector
func (c *UPnPCollector) Collect(ch chan<- prometheus.Metric) {
	mappings, err := getUPnPMappings()
	if err != nil {
		log.Printf("error collecting upnp metrics: %v", err)
		return
	}

	// export total mapping count
	ch <- prometheus.MustNewConstMetric(
		c.upnpMappingCount,
		prometheus.GaugeValue,
		float64(len(mappings)),
	)

	for _, mapping := range mappings {
		// mapping info as a constant metric with value 1
		ch <- prometheus.MustNewConstMetric(
			c.upnpInfo,
			prometheus.GaugeValue,
			1,
			mapping.Protocol,
			mapping.ExternalPort,
			mapping.InternalIP,
			mapping.InternalPort,
			mapping.Description,
		)

		// lease duration
		ch <- prometheus.MustNewConstMetric(
			c.upnpLeaseSeconds,
			prometheus.GaugeValue,
			mapping.LeaseSeconds,
			mapping.Protocol,
			mapping.ExternalPort,
			mapping.InternalIP,
			mapping.InternalPort,
			mapping.Description,
		)
	}
}

// UPnP port mapping information
type UPnPMapping struct {
	Protocol     string
	ExternalPort string
	InternalIP   string
	InternalPort string
	LeaseSeconds float64
	Description  string
}

// get UPnP port mappings from miniupnpd leases file
func getUPnPMappings() ([]UPnPMapping, error) {
	// try common locations for miniupnpd leases file
	leasePaths := []string{
		"/var/run/miniupnpd.leases",
		"/tmp/miniupnpd.leases",
		"/var/lib/miniupnpd/leases",
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

	return parseMiniUPnPDLeases(file)
}

// parse miniupnpd leases file
// format: PROTOCOL:EXT_PORT:INT_IP:INT_PORT:LEASE_DURATION:DESCRIPTION
// or newer format with timestamps: PROTOCOL:EXT_PORT:INT_IP:INT_PORT:TIMESTAMP:LEASE_DURATION:DESCRIPTION
func parseMiniUPnPDLeases(file *os.File) ([]UPnPMapping, error) {
	var mappings []UPnPMapping
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.SplitN(line, ":", 7)

		if len(fields) >= 6 {
			protocol := strings.ToUpper(fields[0])
			externalPort := fields[1]
			internalIP := fields[2]
			internalPort := fields[3]

			var leaseSeconds float64
			var description string

			if len(fields) == 6 {
				// old format: PROTOCOL:EXT_PORT:INT_IP:INT_PORT:LEASE_DURATION:DESCRIPTION
				leaseSeconds, _ = strconv.ParseFloat(fields[4], 64)
				description = fields[5]
			} else if len(fields) >= 7 {
				// newer format: PROTOCOL:EXT_PORT:INT_IP:INT_PORT:TIMESTAMP:LEASE_DURATION:DESCRIPTION
				leaseSeconds, _ = strconv.ParseFloat(fields[5], 64)
				description = fields[6]
			}

			// clean up description
			description = strings.TrimSpace(description)
			if description == "" {
				description = "unknown"
			}

			mappings = append(mappings, UPnPMapping{
				Protocol:     protocol,
				ExternalPort: externalPort,
				InternalIP:   internalIP,
				InternalPort: internalPort,
				LeaseSeconds: leaseSeconds,
				Description:  description,
			})
		}
	}

	return mappings, scanner.Err()
}
