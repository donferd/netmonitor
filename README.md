# NetMonitor

NetMonitor is a lightweight network monitoring daemon written in Go.  
It continuously pings host(s), tracks uptime, latency, jitter, and exposes real-time metrics via a web dashboard and JSON API.

This is my first project in Go. Learning with AI help to build and understand how to set up and use GO.

I wanted to keep track of all possible problems with network latency and how consistent the host responds.
You can input an unlimited number of hosts and how often you want them to be pinged if you don't like the default value.

Also you can change the default port to look at if you don't like 8080. 

Enjoy.

---

## 🚀 Features

- ICMP ping monitoring (latency, packet loss)
- Tracks jitter, min/max/avg latency
- Exposes a live HTML dashboard at `/`
- JSON API at `/api/stats`
- Can run as a Linux daemon (systemd service)

---

## 📦 Installation

```bash
git clone https://github.com/donferd/netmonitor
cd netmonitor
go mod tidy
go build -o netmonitor ./cmd/netmonitor
sudo mv netmonitor /usr/local/bin/
