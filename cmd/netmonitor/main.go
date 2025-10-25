package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type PingStats struct {
	Host           string    `json:"host"`
	Status         string    `json:"status"`
	LastSeen       time.Time `json:"lastSeen"`
	PacketsSent    int       `json:"packetsSent"`
	PacketsRecv    int       `json:"packetsRecv"`
	PacketLoss     float64   `json:"packetLoss"`
	AvgLatency     float64   `json:"avgLatency"`
	MinLatency     float64   `json:"minLatency"`
	MaxLatency     float64   `json:"maxLatency"`
	CurrentLatency float64   `json:"currentLatency"`
	Jitter         float64   `json:"jitter"`
}

type Monitor struct {
	hosts    []string
	port     int
	interval time.Duration
	stats    map[string]*PingStats
	mu       sync.RWMutex
}

func NewMonitor(hosts []string, port int, interval time.Duration) *Monitor {
	m := &Monitor{
		hosts:    hosts,
		port:     port,
		interval: interval,
		stats:    make(map[string]*PingStats),
	}

	for _, host := range hosts {
		m.stats[host] = &PingStats{
			Host:       host,
			Status:     "unknown",
			MinLatency: -1,
			MaxLatency: -1,
		}
	}

	return m
}

func (m *Monitor) ping(host string) (float64, error) {
	// Resolve the host
	addr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return 0, err
	}

	// Create ICMP connection
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Set timeout
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	// Create ICMP message
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   1,
			Seq:  1,
			Data: []byte("PING"),
		},
	}

	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}

	// Send ping
	start := time.Now()
	_, err = conn.WriteTo(msgBytes, addr)
	if err != nil {
		return 0, err
	}

	// Wait for reply
	reply := make([]byte, 1500)
	_, _, err = conn.ReadFrom(reply)
	if err != nil {
		return 0, err
	}

	duration := time.Since(start)
	return duration.Seconds() * 1000, nil // Return in milliseconds
}

func (m *Monitor) monitorHost(host string) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	var lastLatency float64

	for range ticker.C {
		latency, err := m.ping(host)

		m.mu.Lock()
		stats := m.stats[host]
		stats.PacketsSent++

		if err != nil {
			stats.Status = "down"
		} else {
			stats.Status = "up"
			stats.PacketsRecv++
			stats.LastSeen = time.Now()
			stats.CurrentLatency = latency

			// Update min/max
			if stats.MinLatency == -1 || latency < stats.MinLatency {
				stats.MinLatency = latency
			}
			if latency > stats.MaxLatency {
				stats.MaxLatency = latency
			}

			// Calculate average latency
			if stats.PacketsRecv == 1 {
				stats.AvgLatency = latency
			} else {
				stats.AvgLatency = (stats.AvgLatency*float64(stats.PacketsRecv-1) + latency) / float64(stats.PacketsRecv)
			}

			// Calculate jitter (variance in latency)
			if lastLatency > 0 {
				jitter := latency - lastLatency
				if jitter < 0 {
					jitter = -jitter
				}
				stats.Jitter = (stats.Jitter*0.9 + jitter*0.1) // Exponential moving average
			}
			lastLatency = latency
		}

		// Calculate packet loss
		if stats.PacketsSent > 0 {
			stats.PacketLoss = float64(stats.PacketsSent-stats.PacketsRecv) / float64(stats.PacketsSent) * 100
		}

		m.mu.Unlock()
	}
}

func (m *Monitor) Start() {
	for _, host := range m.hosts {
		go m.monitorHost(host)
	}
}

func (m *Monitor) GetStats() []PingStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PingStats, 0, len(m.stats))
	for _, stats := range m.stats {
		result = append(result, *stats)
	}
	return result
}

func (m *Monitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/stats" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.GetStats())
		return
	}

	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, htmlPage)
		return
	}

	http.NotFound(w, r)
}

const htmlPage = `<!DOCTYPE html>
<html>
<head>
    <title>Network Monitor</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        h1 {
            color: #333;
            margin-bottom: 30px;
        }
        .host-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
            gap: 20px;
        }
        .host-card {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            transition: box-shadow 0.3s;
        }
        .host-card:hover {
            box-shadow: 0 4px 8px rgba(0,0,0,0.15);
        }
        .host-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
            padding-bottom: 15px;
            border-bottom: 2px solid #f0f0f0;
        }
        .host-name {
            font-size: 18px;
            font-weight: bold;
            color: #333;
        }
        .status {
            padding: 5px 15px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: bold;
            text-transform: uppercase;
        }
        .status.up {
            background: #4caf50;
            color: white;
        }
        .status.down {
            background: #f44336;
            color: white;
        }
        .status.unknown {
            background: #999;
            color: white;
        }
        .metric {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            border-bottom: 1px solid #f5f5f5;
        }
        .metric-label {
            color: #666;
            font-size: 14px;
        }
        .metric-value {
            font-weight: bold;
            color: #333;
            font-size: 14px;
        }
        .metric-value.good {
            color: #4caf50;
        }
        .metric-value.warning {
            color: #ff9800;
        }
        .metric-value.bad {
            color: #f44336;
        }
        .last-update {
            text-align: center;
            color: #999;
            margin-top: 20px;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Network Monitor</h1>
        <div class="host-grid" id="hostGrid"></div>
        <div class="last-update" id="lastUpdate"></div>
    </div>

    <script>
        function formatLatency(ms) {
            return ms > 0 ? ms.toFixed(2) + ' ms' : 'N/A';
        }

        function formatPacketLoss(loss) {
            return loss.toFixed(2) + '%';
        }

        function getLatencyClass(latency) {
            if (latency < 0) return '';
            if (latency < 50) return 'good';
            if (latency < 100) return 'warning';
            return 'bad';
        }

        function getPacketLossClass(loss) {
            if (loss === 0) return 'good';
            if (loss < 5) return 'warning';
            return 'bad';
        }

        function formatLastSeen(timestamp) {
            if (!timestamp || timestamp === '0001-01-01T00:00:00Z') return 'Never';
            const date = new Date(timestamp);
            const now = new Date();
            const diff = Math.floor((now - date) / 1000);
            
            if (diff < 60) return diff + 's ago';
            if (diff < 3600) return Math.floor(diff / 60) + 'm ago';
            return Math.floor(diff / 3600) + 'h ago';
        }

        function updateStats() {
            fetch('/api/stats')
                .then(response => response.json())
                .then(data => {
                    const grid = document.getElementById('hostGrid');
                    grid.innerHTML = '';
                    
                    data.forEach(host => {
                        const card = document.createElement('div');
                        card.className = 'host-card';
                        card.innerHTML = 
                            '<div class="host-header">' +
                                '<div class="host-name">' + host.host + '</div>' +
                                '<div class="status ' + host.status + '">' + host.status + '</div>' +
                            '</div>' +
                            '<div class="metric">' +
                                '<span class="metric-label">Current Latency</span>' +
                                '<span class="metric-value ' + getLatencyClass(host.currentLatency) + '">' + formatLatency(host.currentLatency) + '</span>' +
                            '</div>' +
                            '<div class="metric">' +
                                '<span class="metric-label">Average Latency</span>' +
                                '<span class="metric-value ' + getLatencyClass(host.avgLatency) + '">' + formatLatency(host.avgLatency) + '</span>' +
                            '</div>' +
                            '<div class="metric">' +
                                '<span class="metric-label">Min / Max Latency</span>' +
                                '<span class="metric-value">' + formatLatency(host.minLatency) + ' / ' + formatLatency(host.maxLatency) + '</span>' +
                            '</div>' +
                            '<div class="metric">' +
                                '<span class="metric-label">Jitter</span>' +
                                '<span class="metric-value">' + formatLatency(host.jitter) + '</span>' +
                            '</div>' +
                            '<div class="metric">' +
                                '<span class="metric-label">Packet Loss</span>' +
                                '<span class="metric-value ' + getPacketLossClass(host.packetLoss) + '">' + formatPacketLoss(host.packetLoss) + '</span>' +
                            '</div>' +
                            '<div class="metric">' +
                                '<span class="metric-label">Packets Sent / Received</span>' +
                                '<span class="metric-value">' + host.packetsSent + ' / ' + host.packetsRecv + '</span>' +
                            '</div>' +
                            '<div class="metric">' +
                                '<span class="metric-label">Last Seen</span>' +
                                '<span class="metric-value">' + formatLastSeen(host.lastSeen) + '</span>' +
                            '</div>';
                        grid.appendChild(card);
                    });
                    
                    document.getElementById('lastUpdate').textContent = 'Last updated: ' + new Date().toLocaleTimeString();
                })
                .catch(error => console.error('Error fetching stats:', error));
        }

        // Update every 2 seconds
        updateStats();
        setInterval(updateStats, 2000);
    </script>
</body>
</html>`

func main() {
	hostsFlag := flag.String("hosts", "", "Comma-separated list of hosts to monitor")
	portFlag := flag.Int("port", 8080, "Port for the web server")
	intervalFlag := flag.Duration("interval", 5*time.Second, "Ping interval (e.g., 5s, 1m)")

	flag.Parse()

	if *hostsFlag == "" {
		log.Fatal("Error: -hosts flag is required (comma-separated list of hosts)")
	}

	hosts := strings.Split(*hostsFlag, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}

	fmt.Printf("Starting Network Monitor\n")
	fmt.Printf("Monitoring hosts: %v\n", hosts)
	fmt.Printf("Ping interval: %v\n", *intervalFlag)
	fmt.Printf("Web server port: %d\n", *portFlag)
	fmt.Println("\nNote: This program requires raw socket access. Run with sudo if needed.")

	monitor := NewMonitor(hosts, *portFlag, *intervalFlag)
	monitor.Start()

	addr := fmt.Sprintf(":%d", *portFlag)
	fmt.Printf("\nWeb interface available at: http://localhost%s\n", addr)

	log.Fatal(http.ListenAndServe(addr, monitor))
}
