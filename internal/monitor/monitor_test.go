package monitor

import (
	"strings"
	"testing"
)

type fakeClient struct{ out string }

func (f fakeClient) Output(cmd string) (string, error) { return f.out, nil }

const sample = `web-01
Ubuntu 24.04.1 LTS
4h 30m
0.42 0.51 0.47
12.3
524288000 1677721600
51200000 100000000
123456789 987654321
38`

func TestCollectParses(t *testing.T) {
	st, err := Collect(fakeClient{out: sample})
	if err != nil {
		t.Fatal(err)
	}
	if st.Hostname != "web-01" {
		t.Errorf("hostname: %s", st.Hostname)
	}
	if st.CPU != 12.3 {
		t.Errorf("cpu: %f", st.CPU)
	}
	if st.MemUsed != 524288000 || st.MemTotal != 1677721600 {
		t.Errorf("mem: %d/%d", st.MemUsed, st.MemTotal)
	}
	if st.DiskTotal != 100000000 {
		t.Errorf("disk total: %d", st.DiskTotal)
	}
	if st.NetRX != 123456789 || st.NetTX != 987654321 {
		t.Errorf("net: %d/%d", st.NetRX, st.NetTX)
	}
	if st.RunningSvcs != 38 {
		t.Errorf("services: %d", st.RunningSvcs)
	}
	if st.Load1 != 0.42 || st.Load5 != 0.51 {
		t.Errorf("load: %f/%f", st.Load1, st.Load5)
	}
}

func TestMemPct(t *testing.T) {
	st := &Stats{MemUsed: 800, MemTotal: 1600}
	if st.MemPct() != 50 {
		t.Errorf("mem pct: %f", st.MemPct())
	}
}

func TestHealthThresholds(t *testing.T) {
	st := &Stats{DiskUsed: 90, DiskTotal: 100, MemUsed: 50, MemTotal: 100, Load1: 0.1}
	ok, problems := st.Health(80, 90)
	if ok || len(problems) == 0 {
		t.Errorf("expected disk problem: %v %v", ok, problems)
	}
	if !strings.Contains(problems[0], "disk") {
		t.Errorf("wrong problem: %v", problems)
	}
}

func TestCollectBadOutput(t *testing.T) {
	_, err := Collect(fakeClient{out: "too short"})
	if err == nil {
		t.Error("expected error on short output")
	}
}
