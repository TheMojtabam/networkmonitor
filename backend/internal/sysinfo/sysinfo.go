package sysinfo

import (
	"bufio"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type SysInfo struct {
	Hostname   string    `json:"hostname"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
	GoVersion  string    `json:"goVersion"`
	UptimeSec  int64     `json:"uptimeSec"`
	Load1      float64   `json:"load1"`
	Load5      float64   `json:"load5"`
	Load15     float64   `json:"load15"`
	MemTotalKB int64     `json:"memTotalKB"`
	MemAvailKB int64     `json:"memAvailKB"`
	TS         time.Time `json:"ts"`
}

func Collect() (SysInfo, error) {
	h, _ := os.Hostname()
	up, _ := readUptime()
	l1, l5, l15, _ := readLoadavg()
	mt, ma, _ := readMeminfo()

	return SysInfo{
		Hostname:   h,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		GoVersion:  runtime.Version(),
		UptimeSec:  up,
		Load1:      l1,
		Load5:      l5,
		Load15:     l15,
		MemTotalKB: mt,
		MemAvailKB: ma,
		TS:         time.Now().UTC(),
	}, nil
}

func readUptime() (int64, error) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return 0, errors.New("bad /proc/uptime")
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

func readLoadavg() (float64, float64, float64, error) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return 0, 0, 0, errors.New("bad /proc/loadavg")
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15, nil
}

func readMeminfo() (totalKB, availKB int64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				totalKB, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				availKB, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}
	}
	return totalKB, availKB, s.Err()
}
