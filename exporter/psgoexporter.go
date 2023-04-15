package exporter

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type PsgoExporter struct {
	cpu        *prometheus.GaugeVec
	usr        *prometheus.GaugeVec
	sys        *prometheus.GaugeVec
	iowait     *prometheus.GaugeVec
	numThreads *prometheus.GaugeVec
	vmsKb      *prometheus.GaugeVec
	rssKb      *prometheus.GaugeVec
}

func NewPsgoExporter() *PsgoExporter {
	return &PsgoExporter{
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

func (e *PsgoExporter) Describe(ch chan<- *prometheus.Desc) {
	e.cpu.Describe(ch)
	e.usr.Describe(ch)
	e.sys.Describe(ch)
	e.iowait.Describe(ch)
	e.numThreads.Describe(ch)
	e.vmsKb.Describe(ch)
	e.rssKb.Describe(ch)
}

func (e *PsgoExporter) Collect(ch chan<- prometheus.Metric) {
	e.cpu.Collect(ch)
	e.usr.Collect(ch)
	e.sys.Collect(ch)
	e.iowait.Collect(ch)
	e.numThreads.Collect(ch)
	e.vmsKb.Collect(ch)
	e.rssKb.Collect(ch)
}

func (psgo *PsgoExporter) Set(tags map[string]string, vals map[string]float64, ts time.Time) {
	labels := prometheus.Labels{"hostname": tags["hostname"], "nickname": tags["nickname"], "pid": tags["pid"]}
	psgo.cpu.With(labels).Set(vals["cpuTotal"])
	psgo.usr.With(labels).Set(vals["cpuUsr"])
	psgo.sys.With(labels).Set(vals["cpuSys"])
	psgo.iowait.With(labels).Set(vals["cpuIow"])
	psgo.numThreads.With(labels).Set(vals["numThreads"])
	psgo.vmsKb.With(labels).Set(vals["vmsKb"])
	psgo.rssKb.With(labels).Set(vals["rssKb"])
}
