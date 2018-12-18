// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	_ "net/http/pprof"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var gmonitor []string

type metrics map[int]*prometheus.GaugeVec

func (m metrics) String() string {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = strconv.Itoa(k)
	}
	return strings.Join(s, ",")
}

// Exporter collects HAProxy stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	mutex sync.RWMutex

	name   string
	duDesc *prometheus.Desc
}

// NewExporter returns an initialized Exporter.
func NewExporter(name *string) (*Exporter, error) {

	return &Exporter{

		duDesc: prometheus.NewDesc(
			prometheus.BuildFQName(*name, "folder", "size_bytes"),
			"folder size in bytes.",
			[]string{"name"}, nil,
		),
	}, nil
}

// Describe describes all the metrics ever exported by the HAProxy exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.duDesc
}

func (e *Exporter) FolderUsage() map[string]int {
	var du []byte
	var err error
	var cmd *exec.Cmd
	fileSize := make(map[string]int)

	for _, path := range gmonitor {
		cmd = exec.Command("du", "-k", "-d", "1", path)
		if du, err = cmd.Output(); err != nil {
			fmt.Println(err)
			return fileSize

		}


		stringArray := strings.Split(string(du), "\n")
		for _, value := range stringArray {
			if value == "" {
				continue
			}
			nameValue := strings.Split(value, "\t")
			i, _ := strconv.Atoi(nameValue[0])
			fileSize[nameValue[1]] = i
		}
	}
		// 执行单个shell命令时, 直接运行即可
	return fileSize
}

// Collect fetches the stats from configured HAProxy location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	fileSizeMap := e.FolderUsage()

	for file := range fileSizeMap {
		ch <- prometheus.MustNewConstMetric(
			e.duDesc, prometheus.GaugeValue,
			float64(fileSizeMap[file]), file,
		)
	}

}

func main() {
	const pidFileHelpText = `Path to HAProxy pid file.

	If provided, the standard process metrics get exported for the HAProxy
	process, prefixed with 'haproxy_process_...'. The haproxy_process exporter
	needs to have read access to files owned by the HAProxy process. Depends on
	the availability of /proc.

	https://prometheus.io/docs/instrumenting/writing_clientlibs/#process-metrics.`

	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9101").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	)
	monitor := kingpin.Flag("monitor", "monitor path").Default(".").Strings()
	kingpin.Parse()

	log.Infoln("monitor on ", monitor)
	gmonitor = *monitor

	var name = "file_size"

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("haproxy_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting haproxy_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter, err := NewExporter(&name)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector(name))

	log.Infoln("Listening on", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Haproxy Exporter</title></head>
             <body>
             <h1>Haproxy Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
