package exporter

import (
	//"fmt"
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type psgoExporter struct {
	cpu        *prometheus.GaugeVec
	usr        *prometheus.GaugeVec
	sys        *prometheus.GaugeVec
	iowait     *prometheus.GaugeVec
	numThreads *prometheus.GaugeVec
	vmsKb      *prometheus.GaugeVec
	rssKb      *prometheus.GaugeVec
}

func newPsgoExporter() *psgoExporter {
	return &psgoExporter{
		cpu: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cpu",
				Help: "CPU usage",
			},
			[]string{"hostname", "nickname", "pid"},
		),
		usr: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "usr",
				Help: "User time",
			},
			[]string{"hostname", "nickname", "pid"},
		),
		sys: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sys",
				Help: "System time",
			},
			[]string{"hostname", "nickname", "pid"},
		),
		iowait: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "iowait",
				Help: "I/O wait time",
			},
			[]string{"hostname", "nickname", "pid"},
		),
		numThreads: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "num_threads",
				Help: "Number of threads",
			},
			[]string{"hostname", "nickname", "pid"},
		),
		vmsKb: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vms_kb",
				Help: "Virtual memory size in KB",
			},
			[]string{"hostname", "nickname", "pid"},
		),
		rssKb: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rss_kb",
				Help: "Resident set size in KB",
			},
			[]string{"hostname", "nickname", "pid"},
		),
	}
}

func (e *psgoExporter) Describe(ch chan<- *prometheus.Desc) {
	e.cpu.Describe(ch)
	e.usr.Describe(ch)
	e.sys.Describe(ch)
	e.iowait.Describe(ch)
	e.numThreads.Describe(ch)
	e.vmsKb.Describe(ch)
	e.rssKb.Describe(ch)
}

func (e *psgoExporter) Collect(ch chan<- prometheus.Metric) {
	e.cpu.Collect(ch)
	e.usr.Collect(ch)
	e.sys.Collect(ch)
	e.iowait.Collect(ch)
	e.numThreads.Collect(ch)
	e.vmsKb.Collect(ch)
	e.rssKb.Collect(ch)
}

update

func updateMetrics(psgo *psgoExporter) {
		labels := prometheus.Labels{"hostname": hostname, "nickname": nickname, "pid": pid}
		psgo.cpu.With(labels).Set(cpu)
		psgo.usr.With(labels).Set(usr)
		psgo.sys.With(labels).Set(sys)
		psgo.iowait.With(labels).Set(iowait)
		psgo.numThreads.With(labels).Set(numThreads)
		psgo.vmsKb.With(labels).Set(vmsKb)
		psgo.rssKb.With(labels).Set(rssKb)
	}
}

func startHtml(listenAddress string) {
	psgo := newPsgoExporter()
	prometheus.MustRegister(psgo)

	updateMetrics(*logFilePath, psgo, *backfill)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>PSGO Exporter</title></head>
			<body>
			<h1>PSGO Exporter</h1>
			<p><a href="/metrics">Metrics</a></p>
			</body>
			</html>`))
	})

}
