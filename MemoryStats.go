package agento

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/influxdb/influxdb/client"
)

type MemoryStats struct {
	Used     int64 `json:"u"`
	Free     int64 `json:"f"`
	Shared   int64 `json:"s"`
	Buffers  int64 `json:"b"`
	Cached   int64 `json:"c"`
	SwapUsed int64 `json:"su"`
	SwapFree int64 `json:"sf"`
}

func getMemInfo() *map[string]int64 {
	m := make(map[string]int64)

	path := filepath.Join("/proc/meminfo")
	file, err := os.Open(path)
	if err != nil {
		return &m
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()

		n := strings.Index(text, ":")
		if n == -1 {
			continue
		}

		key := text[:n]
		data := strings.Split(strings.Trim(text[(n+1):], " "), " ")
		if len(data) == 1 {
			value, err := strconv.ParseInt(data[0], 10, 64)
			if err != nil {
				continue
			}
			m[key] = value
		} else if len(data) == 2 {
			if data[1] == "kB" {
				value, err := strconv.ParseInt(data[0], 10, 64)
				if err != nil {
					continue
				}

				m[key] = value
			}
		}
	}

	return &m
}

func GetMemoryStats() *MemoryStats {
	stat := MemoryStats{}
	meminfo := getMemInfo()

	stat.Used = (*meminfo)["MemTotal"] - (*meminfo)["MemFree"] - (*meminfo)["Buffers"] - (*meminfo)["Cached"]
	stat.Free = (*meminfo)["MemFree"]
	stat.Shared = (*meminfo)["Shmem"]
	stat.Buffers = (*meminfo)["Buffers"]
	stat.Cached = (*meminfo)["Cached"]

	stat.SwapUsed = (*meminfo)["SwapTotal"] - (*meminfo)["SwapFree"]
	stat.SwapFree = (*meminfo)["SwapFree"]

	return &stat
}

func (s *MemoryStats) GetPoints() []client.Point {
	points := make([]client.Point, 7)

	points[0] = SimplePoint("mem.Used", s.Used)
	points[1] = SimplePoint("mem.Free", s.Free)
	points[2] = SimplePoint("mem.Shared", s.Shared)
	points[3] = SimplePoint("mem.Buffers", s.Buffers)
	points[4] = SimplePoint("mem.Cached", s.Cached)
	points[5] = SimplePoint("swap.Used", s.SwapUsed)
	points[6] = SimplePoint("swap.Free", s.SwapFree)

	return points
}

func (s *MemoryStats) GetDoc(m map[string]string) {
	m["mem.Used"] = "Memory used (b)"
	m["mem.Free"] = "Free memory (b)"
	m["mem.Shared"] = "Memory shared among multiple processes (b)"
	m["mem.Buffers"] = "Memory used for buffers (b)"
	m["mem.Cached"] = "Memory used for cache (b)"
	m["swap.Used"] = "Used swap (b)"
	m["swap.Free"] = "Free swap (b)"
}
