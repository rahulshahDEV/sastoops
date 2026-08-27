package monitor

import (
	"fmt"
	"strconv"
	"strings"
)

type Client interface {
	Output(command string) (string, error)
}

type Stats struct {
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	CPU         float64 `json:"cpu_percent"`
	MemUsed     uint64  `json:"mem_used"`
	MemTotal    uint64  `json:"mem_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskTotal   uint64  `json:"disk_total"`
	Load1       float64 `json:"load_1"`
	Load5       float64 `json:"load_5"`
	Uptime      string  `json:"uptime"`
	NetRX       uint64  `json:"net_rx"`
	NetTX       uint64  `json:"net_tx"`
	RunningSvcs int     `json:"running_services"`
}

const script = `set -e
HOST=$(hostname)
OS=$(. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME" || echo unknown)
UP=$(uptime -p | sed 's/up //')
LOAD=$(awk '{print $1, $2, $3}' /proc/loadavg)
CPU1=$(awk '{u+=$2+$3; s+=$4; n+=$5} END {print u+s, n}' /proc/stat)
sleep 0.5
CPU2=$(awk '{u+=$2+$3; s+=$4; n+=$5} END {print u+s, n}' /proc/stat)
CPU=$(awk -v a="$CPU1" -v b="$CPU2" 'BEGIN{split(a,A); split(b,B); d1=B[1]-A[1]; d2=B[2]-A[2]; if(d2>0) printf "%.1f", 100*(d1/d2); else printf "0"}')
MEM=$(awk '/MemTotal:/{t=$2} /MemAvailable:/{a=$2} END {printf "%d %d", (t-a)*1024, t*1024}' /proc/meminfo)
DISK=$(df -B1 / | awk 'NR==2{print $3, $2}')
NET=$(awk '/eth0|ens[0-9]+|enp[0-9]+:/ {print $2, $10; exit}' /proc/net/dev)
SVC=$(systemctl list-units --type=service --state=running --no-legend 2>/dev/null | wc -l)
printf '%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n' "$HOST" "$OS" "$UP" "$LOAD" "$CPU" "$MEM" "$DISK" "$NET" "$SVC"`

func Collect(c Client) (*Stats, error) {
	out, err := c.Output(script)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 9 {
		return nil, fmt.Errorf("unexpected monitor output: %q", out)
	}
	s := &Stats{Hostname: lines[0], OS: lines[1], Uptime: lines[2]}
	load := strings.Fields(lines[3])
	if len(load) >= 2 {
		s.Load1, _ = strconv.ParseFloat(load[0], 64)
		s.Load5, _ = strconv.ParseFloat(load[1], 64)
	}
	s.CPU, _ = strconv.ParseFloat(lines[4], 64)
	fmt.Sscanf(lines[5], "%d %d", &s.MemUsed, &s.MemTotal)
	fmt.Sscanf(lines[6], "%d %d", &s.DiskUsed, &s.DiskTotal)
	net := strings.Fields(lines[7])
	if len(net) >= 2 {
		s.NetRX, _ = strconv.ParseUint(net[0], 10, 64)
		s.NetTX, _ = strconv.ParseUint(net[1], 10, 64)
	}
	s.RunningSvcs, _ = strconv.Atoi(lines[8])
	return s, nil
}

func (s *Stats) MemPct() float64 {
	if s.MemTotal == 0 {
		return 0
	}
	return float64(s.MemUsed) / float64(s.MemTotal) * 100
}

func (s *Stats) DiskPct() float64 {
	if s.DiskTotal == 0 {
		return 0
	}
	return float64(s.DiskUsed) / float64(s.DiskTotal) * 100
}

// HealthReport checks thresholds; returns ok + problems.
func (s *Stats) Health(diskPct, memPct float64) (bool, []string) {
	var problems []string
	if s.DiskPct() > diskPct {
		problems = append(problems, fmt.Sprintf("disk at %.0f%% (threshold %.0f%%)", s.DiskPct(), diskPct))
	}
	if s.MemPct() > memPct {
		problems = append(problems, fmt.Sprintf("memory at %.0f%% (threshold %.0f%%)", s.MemPct(), memPct))
	}
	if s.Load1 > float64(4) {
		problems = append(problems, fmt.Sprintf("load high: %.2f", s.Load1))
	}
	return len(problems) == 0, problems
}

// TopServices returns running service names.
func TopServices(c Client, n int) ([]string, error) {
	out, err := c.Output(fmt.Sprintf("systemctl list-units --type=service --state=running --no-legend --no-pager | awk '{print $1}' | head -%d", n))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	var svcs []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			svcs = append(svcs, l)
		}
	}
	return svcs, nil
}
