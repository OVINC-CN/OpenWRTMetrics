# OpenWRT Prometheus Exporter

A Prometheus exporter for OpenWRT routers that provides network interface and connected device metrics.

## Features

- **Network Interface Metrics**:
  - Interface name
  - Cumulative uptime
  - Total bytes received/transmitted
  - Total packets received/transmitted

- **Connected Device Metrics**:
  - Device hostname
  - Assigned internal IP address
  - MAC address
  - DHCP lease remaining time

- **Ping Metrics**:
  - Ping latency (min/avg/max) in milliseconds
  - Packet loss percentage
  - Packets sent/received
  - Support for multiple targets
  - Configurable ping count, interval, and timeout
  - Uses pro-bing library for cross-platform ICMP/UDP ping (no external ping command required)

- **UPnP Metrics**:
  - Active UPnP port mapping information
  - Port mapping lease duration
  - Total number of active mappings
  - Protocol, external/internal ports, internal IP, and description labels

## Installation

### Build from source

```bash
go build -o openwrt-exporter
```

### Cross-compile for OpenWRT (MIPS)

```bash
# For MIPS (big endian)
GOOS=linux GOARCH=mips go build -o openwrt-exporter

# For MIPS (little endian)
GOOS=linux GOARCH=mipsle go build -o openwrt-exporter

# For ARM
GOOS=linux GOARCH=arm GOARM=7 go build -o openwrt-exporter

# For ARM64
GOOS=linux GOARCH=arm64 go build -o openwrt-exporter
```

## Usage

### Run the exporter

```bash
./openwrt-exporter
```

By default, the exporter listens on port `9101` and exposes metrics at `/metrics`.

### Command-line options

```bash
./openwrt-exporter -listen-address=":9101" -metrics-path="/metrics"
```

- `-listen-address`: Address to listen on for metrics (default: `:9101`)
- `-metrics-path`: Path under which to expose metrics (default: `/metrics`)

### Environment Variables

The ping collector supports the following environment variables:

- `PING_TARGETS`: Comma-separated list of ping targets (IP addresses or hostnames)
  - Example: `PING_TARGETS="8.8.8.8,1.1.1.1,google.com"`
- `PING_COUNT`: Number of ping packets to send per target (default: `10`)
- `PING_INTERVAL`: Interval between ping packets in seconds (default: `10ms`)
- `PING_TIMEOUT`: Ping timeout in seconds (default: `3s`)

Example with ping configuration:

```bash
PING_TARGETS="8.8.8.8,1.1.1.1" PING_COUNT=5 PING_TIMEOUT=3 ./openwrt-exporter
```

### Access metrics

```bash
curl http://localhost:9101/metrics
```

## Metrics

### Network Interface Metrics

```
# HELP openwrt_network_receive_bytes_total total number of bytes received on network interface
# TYPE openwrt_network_receive_bytes_total counter
openwrt_network_receive_bytes_total{interface="eth0"} 1.23456789e+09

# HELP openwrt_network_transmit_bytes_total total number of bytes transmitted on network interface
# TYPE openwrt_network_transmit_bytes_total counter
openwrt_network_transmit_bytes_total{interface="eth0"} 9.87654321e+08

# HELP openwrt_network_receive_packets_total total number of packets received on network interface
# TYPE openwrt_network_receive_packets_total counter
openwrt_network_receive_packets_total{interface="eth0"} 1234567

# HELP openwrt_network_transmit_packets_total total number of packets transmitted on network interface
# TYPE openwrt_network_transmit_packets_total counter
openwrt_network_transmit_packets_total{interface="eth0"} 987654

# HELP openwrt_network_uptime_seconds network interface uptime in seconds
# TYPE openwrt_network_uptime_seconds gauge
openwrt_network_uptime_seconds{interface="eth0"} 86400
```

### Connected Device Metrics

```
# HELP openwrt_device_info information about connected devices
# TYPE openwrt_device_info gauge
openwrt_device_info{hostname="my-phone",ip="192.168.1.100",mac="aa:bb:cc:dd:ee:ff"} 1

# HELP openwrt_device_dhcp_lease_remaining_seconds dhcp lease remaining time in seconds
# TYPE openwrt_device_dhcp_lease_remaining_seconds gauge
openwrt_device_dhcp_lease_remaining_seconds{hostname="my-phone",ip="192.168.1.100",mac="aa:bb:cc:dd:ee:ff"} 3600
```

### Ping Metrics

```
# HELP openwrt_ping_avg_latency_ms average ping latency in milliseconds
# TYPE openwrt_ping_avg_latency_ms gauge
openwrt_ping_avg_latency_ms{target="8.8.8.8"} 12.345

# HELP openwrt_ping_min_latency_ms minimum ping latency in milliseconds
# TYPE openwrt_ping_min_latency_ms gauge
openwrt_ping_min_latency_ms{target="8.8.8.8"} 10.123

# HELP openwrt_ping_max_latency_ms maximum ping latency in milliseconds
# TYPE openwrt_ping_max_latency_ms gauge
openwrt_ping_max_latency_ms{target="8.8.8.8"} 15.678

# HELP openwrt_ping_packet_loss_percent ping packet loss percentage
# TYPE openwrt_ping_packet_loss_percent gauge
openwrt_ping_packet_loss_percent{target="8.8.8.8"} 0

# HELP openwrt_ping_packets_sent_total total number of ping packets sent
# TYPE openwrt_ping_packets_sent_total counter
openwrt_ping_packets_sent_total{target="8.8.8.8"} 3

# HELP openwrt_ping_packets_received_total total number of ping packets received
# TYPE openwrt_ping_packets_received_total counter
openwrt_ping_packets_received_total{target="8.8.8.8"} 3
```

### UPnP Metrics

```
# HELP openwrt_upnp_mapping_count total number of active UPnP port mappings
# TYPE openwrt_upnp_mapping_count gauge
openwrt_upnp_mapping_count 2

# HELP openwrt_upnp_mapping_info information about UPnP port mappings
# TYPE openwrt_upnp_mapping_info gauge
openwrt_upnp_mapping_info{protocol="TCP",external_port="12345",internal_ip="192.168.1.100",internal_port="12345",description="My App"} 1

# HELP openwrt_upnp_mapping_lease_seconds UPnP port mapping lease duration in seconds (0 means permanent)
# TYPE openwrt_upnp_mapping_lease_seconds gauge
openwrt_upnp_mapping_lease_seconds{protocol="TCP",external_port="12345",internal_ip="192.168.1.100",internal_port="12345",description="My App"} 86400
```

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'openwrt'
    static_configs:
      - targets: ['<openwrt-router-ip>:9101']
```

## Running as a Service on OpenWRT

Create a service file `/etc/init.d/openwrt-exporter`:

```bash
#!/bin/sh /etc/rc.common

# openwrt init script for prometheus exporter

START=99
STOP=10

USE_PROCD=1

PROG=/usr/bin/openwrt-exporter
LISTEN_ADDRESS=":9101"
METRICS_PATH="/metrics"

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/openwrt-exporter
    procd_set_param env PING_TARGETS="1.1.1.1" \
                        PING_COUNT=10 \
                        PING_INTERVAL=10ms \
                        PING_TIMEOUT=3s
    procd_set_param user root
    procd_set_param respawn
    procd_set_param stderr 1
    procd_set_param stdout 1
    procd_close_instance
}

stop_service() {
    service_stop $PROG
}

reload_service() {
    stop_service
    start_service
}
```

Enable and start the service:

```bash
chmod +x /etc/init.d/openwrt-exporter
/etc/init.d/openwrt-exporter enable
/etc/init.d/openwrt-exporter start
```

## Requirements

- Go 1.21 or higher (for building)
- OpenWRT router with:
  - `/proc/net/dev` for network interface statistics
  - `/tmp/dhcp.leases` or `/var/lib/misc/dnsmasq.leases` for DHCP leases
  - `/proc/net/arp` or `ip neigh` command for ARP table
  - `miniupnpd` package for UPnP metrics (optional, leases file at `/var/run/miniupnpd.leases`)

## License

See [LICENSE](LICENSE) file.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
