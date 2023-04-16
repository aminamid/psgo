package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/VictoriaMetrics/metrics"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var (
	//go:embed version.txt
	version string

	reduceCfg     string
	showVersion   bool
	optInterval   int
	optLenCmdline int
	regxCfg       string
	listenAddress string
	metricsMutex  sync.Mutex
)

func main() {
	flag.BoolVar(&showVersion, "v", false, "show version information")
	flag.IntVar(&optInterval, "i", 10, "interval sec")
	flag.IntVar(&optLenCmdline, "l", 0, "max length to show cmdline")
	flag.StringVar(&regxCfg, "s", `{"NOCMD":"^$","SYSTEMD":"^(/usr)?/lib/systemd","SBIN":"^(/usr)?/sbin","BASH":"^-bash$","MXOS":"^[^ ]*java .*/mxos/server/bin","CASS":"^[^ ]*java .*service.CassandraDaemon"}`, "cmdline regular expression matching for aggregating multiple processes")
	flag.StringVar(&reduceCfg, "a", `["NOCMD","SYSTEMD","SBIN","BASH"]`, "Aggregate statistical information from multiple processes based on their nicknames")
	flag.StringVar(&listenAddress, "u", ":10040", "Listen address")
	flag.Parse()

	if showVersion {
		f1, _ := filepath.Abs(".")
		f2, _ := filepath.Abs("logs")
		fmt.Printf("%s\n%s\n%s\n", version, f1, f2)
		os.Exit(0)
	}
	var regxMap map[string]string
	err := json.Unmarshal([]byte(regxCfg), &regxMap)
	if err != nil {
		log.Fatal(err)
	}

	var reduceList []string
	err = json.Unmarshal([]byte(reduceCfg), &reduceList)
	if err != nil {
		log.Fatal(err)
	}

	go MetricsListen(listenAddress)

	ctx := context.Background()
	phost, err := host.Info()
	if err != nil {
		panic(err)
	}
	interval := time.Duration(optInterval) * time.Second
	tsCh := make(chan time.Time)
	go pacemaker(tsCh, interval)

	statProc := NewStatProc(ctx, phost.Hostname, regxMap)

	headerString := "#time hostname nickname name pid cpu usr sys iowait num_threads VmsKb RssKb cmdline"
	fmt.Println(headerString)

	for {
		tskick := <-tsCh
		time.Sleep(time.Until(tskick))
		ts := tskick.Add(-interval)
		statProc.Update(ts)
		if len(reduceList) > 0 {
			statProc.ReduceSumm(regxMap, reduceList)
		}
		statProc.PrintSumm(optLenCmdline)
		statProc.UpdateMetrics()
		statProc.Reinit()
		fmt.Println(headerString)
	}
}

func pacemaker(tsCh chan time.Time, interval time.Duration) {
	startTime := time.Now().Add(time.Second)
	tsCh <- startTime
	currentTime := startTime
	for {
		tsCh <- currentTime.Add(interval)
		currentTime = currentTime.Add(interval)
	}
}

type StatProc struct {
	newprocs  map[int32]*process.Process
	oldprocs  map[int32]*process.Process
	ctx       context.Context
	regexpMap map[string]*regexp.Regexp
	summ      map[int32]*StatProcSumm
	host      string
}

func (sp *StatProc) Summ() map[int32]*StatProcSumm {
	return sp.summ
}

type StatProcSumm struct {
	ts      time.Time
	tags    map[string]string
	vals    map[string]float64
	cmdline string
}

func (sp *StatProc) AddAndDelete(id1, id2 int32) {
	for k, v := range sp.summ[id2].vals {
		sp.summ[id1].vals[k] += v
	}
	delete(sp.summ, id2)
}
func (sp *StatProc) ReduceSumm(regxCfg map[string]string, reduceList []string) {
	var doReduce bool
	idx := make(map[string]int32)

	for i, v := range sp.summ {
		if v.tags["nickname"] == v.tags["name"] {
			continue
		}
		doReduce = false
		for _, x := range reduceList {
			if x == v.tags["nickname"] {
				doReduce = true
				break
			}
		}
		if !doReduce {
			continue
		}
		if _, ok := idx[v.tags["nickname"]]; !ok {
			idx[v.tags["nickname"]] = i
			sp.summ[i].cmdline = regxCfg[v.tags["nickname"]]
			continue
		}
		sp.AddAndDelete(idx[v.tags["nickname"]], i)
	}
}

func NewStatProcSumm() *StatProcSumm {
	sps := new(StatProcSumm)
	sps.tags = make(map[string]string)
	sps.vals = make(map[string]float64)
	return sps
}

func NewStatProc(ctx context.Context, host string, regxs map[string]string) StatProc {
	var sp StatProc
	sp.ctx = ctx
	sp.host = host
	sp.regexpMap = make(map[string]*regexp.Regexp)
	for k, v := range regxs {
		sp.regexpMap[k] = regexp.MustCompile(v)
	}
	sp.newprocs = make(map[int32]*process.Process)
	sp.oldprocs = make(map[int32]*process.Process)
	pids, err := process.Pids()
	if err != nil {
		pids = make([]int32, 0)
	}
	for _, pid := range pids {
		p, err := process.NewProcessWithContext(sp.ctx, pid)
		if err != nil {
			continue
		}
		sp.oldprocs[pid] = p
	}
	return sp
}
func (sp *StatProc) Update(ts time.Time) {
	sp.summ = make(map[int32]*StatProcSumm)
	pids, err := process.Pids()
	if err != nil {
		pids = make([]int32, 0)
	}
	for _, pid := range pids {
		isold := true
		var p *process.Process
		var ok bool
		if p, ok = sp.oldprocs[pid]; !ok {
			isold = false
			p, err = process.NewProcessWithContext(sp.ctx, pid)
			if err != nil {
				continue
			}
		}
		cmdline, err := p.Cmdline()
		if err != nil {
			if isold {
				delete(sp.oldprocs, pid)
				continue
			}
		}
		//fmt.Printf("### cmdline\n%#v\n", *p)
		pname, err := p.Name()
		if err != nil {
			if isold {
				delete(sp.oldprocs, pid)
				continue
			}
		}
		//fmt.Printf("### name\n%#v\n", *p)
		cpuTotal, cpuUser, cpuSystem, cpuIowait, err := p.PercentAllWithContext(sp.ctx, 0)
		if err != nil {
			if isold {
				delete(sp.oldprocs, pid)
				continue
			}
		}
		//fmt.Printf("### cpu\n%#v\n", *p)
		num_thread, err := p.NumThreads()
		if err != nil {
			if isold {
				delete(sp.oldprocs, pid)
				continue
			}
		}
		//fmt.Printf("### thread\n%#v\n", *p)
		mem, err := p.MemoryInfo()
		if err != nil || mem == nil{
			if isold {
				delete(sp.oldprocs, pid)
				continue
			}
		}
		//fmt.Printf("### mem\n%#v\n###########\n", *p)
		sp.newprocs[pid] = p

		nickname := pname

		for k, v := range sp.regexpMap {
			if v.FindStringIndex(cmdline) != nil {
				nickname = k
			}
		}
		sp.summ[pid] = NewStatProcSumm()

		const unitmem = 1024
		sp.summ[pid].ts = ts
		sp.summ[pid].tags["hostname"] = sp.host
		sp.summ[pid].tags["pid"] = fmt.Sprintf("%d", p.Pid)
		sp.summ[pid].tags["name"] = pname
		sp.summ[pid].tags["nickname"] = nickname
		sp.summ[pid].vals["cpuTotal"] = cpuTotal
		sp.summ[pid].vals["cpuUsr"] = cpuUser
		sp.summ[pid].vals["cpuSys"] = cpuSystem
		sp.summ[pid].vals["cpuIow"] = cpuIowait
		sp.summ[pid].vals["numThreads"] = float64(num_thread)
		sp.summ[pid].vals["vmsKb"] = float64(mem.VMS / unitmem)
		sp.summ[pid].vals["rssKb"] = float64(mem.RSS / unitmem)
		sp.summ[pid].cmdline = cmdline

	}
}
func (sp *StatProc) PrintSumm(lenCmdline int) {
	if lenCmdline < 1 {
		for _, sps := range sp.summ {
			fmt.Printf("%s %s %s %s %s %.1f %.0f %.0f %.0f %.0f %.0f %.0f %s\n", sps.ts.Format("2006-01-02T15:04:05"), sps.tags["hostname"], sps.tags["nickname"], sps.tags["name"], sps.tags["pid"], sps.vals["cpuTotal"], sps.vals["cpuUsr"], sps.vals["cpuSys"], sps.vals["cpuIow"], sps.vals["numThreads"], sps.vals["vmsKb"], sps.vals["rssKb"], sps.cmdline)
		}
	} else {
		var lcmd int
		for _, sps := range sp.summ {
			if len(sps.cmdline) < lenCmdline {
				lcmd = len(sps.cmdline)
			} else {
				lcmd = lenCmdline
			}
			fmt.Printf("%s %s %s %s %s %.1f %.0f %.0f %.0f %.0f %.0f %.0f %s\n", sps.ts.Format("2006-01-02T15:04:05"), sps.tags["hostname"], sps.tags["nickname"], sps.tags["name"], sps.tags["pid"], sps.vals["cpuTotal"], sps.vals["cpuUsr"], sps.vals["cpuSys"], sps.vals["cpuIow"], sps.vals["numThreads"], sps.vals["vmsKb"], sps.vals["rssKb"], sps.cmdline[0:lcmd])
		}
	}
}
func (sp *StatProc) Reinit() {
	fmt.Printf("\n")
	sp.oldprocs = sp.newprocs
	sp.newprocs = make(map[int32]*process.Process)
}

func MetricsListen(listenAddr string) {
	http.HandleFunc("/metrics", metricsHandler)
	http.ListenAndServe(listenAddr, nil)
}

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	metrics.WritePrometheus(w, false)
}

func (sp *StatProc) UpdateMetrics() {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	metrics.UnregisterAllMetrics()

	for _, sps := range sp.summ {
		localSps := sps

		labels := fmt.Sprintf(`hostname="%s",nickname="%s",pid="%s"`, localSps.tags["hostname"], localSps.tags["nickname"], localSps.tags["pid"])

		metrics.NewGauge(fmt.Sprintf(`psgo_cpupercent_cpu{%s}`, labels), func() float64 { return localSps.vals["cpuTotal"] })
		metrics.NewGauge(fmt.Sprintf(`psgo_millicore_usr{%s}`, labels), func() float64 { return localSps.vals["cpuUsr"] })
		metrics.NewGauge(fmt.Sprintf(`psgo_millicore_sys{%s}`, labels), func() float64 { return localSps.vals["cpuSys"] })
		metrics.NewGauge(fmt.Sprintf(`psgo_millicore_iowait{%s}`, labels), func() float64 { return localSps.vals["cpuIow"] })
		metrics.NewGauge(fmt.Sprintf(`psgo_numthreads{%s}`, labels), func() float64 { return localSps.vals["numThreads"] })
		metrics.NewGauge(fmt.Sprintf(`psgo_memkb_vms{%s}`, labels), func() float64 { return localSps.vals["vmsKb"] })
		metrics.NewGauge(fmt.Sprintf(`psgo_memkb_rss{%s}`, labels), func() float64 { return localSps.vals["rssKb"] })
	}
}
