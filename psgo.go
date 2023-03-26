package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
	//"context"
	//"github.com/shirou/gopsutil/v3/cpu"
	//"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
	// "github.com/shirou/gopsutil/mem"  // to use v2
)

var (
	//go:embed version.txt
	version string

	initCfg       bool
	showVersion   bool
	optInterval   int
	optLenCmdline int
	cfgFile       string
)

func main() {
	flag.BoolVar(&showVersion, "v", false, "show version information")
	flag.BoolVar(&initCfg, "init", false, "create default cfgfile if it does not exist")
	flag.IntVar(&optInterval, "i", 10, "interval sec")
	flag.IntVar(&optLenCmdline, "w", -1, "limit length of cmdline")
	flag.StringVar(&cfgFile, "f", "./cfg.yml", "config file")
	flag.Parse()

	if showVersion {
		f1, _ := filepath.Abs(".")
		f2, _ := filepath.Abs("logs")
		fmt.Printf("%s\n%s\n%s\n", version, f1, f2)
		os.Exit(0)
	}

	ctx := context.Background()
	phost, err := host.Info()
	if err != nil {
		panic(err)
	}
	interval := time.Duration(optInterval) * time.Second
	fmt.Printf("time hostname Uid comm pid ppid cpu usr sys iowait num_threads VmSize VmRSS cmdline\n")
	tsCh := make(chan time.Time)
	go pacemaker(tsCh, interval)
	sub1(ctx, phost.Hostname, tsCh, interval)
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
func sub1(ctx context.Context, phost string, tsCh chan time.Time, interval time.Duration) {
	newprocs := make(map[int32]*process.Process)
	oldprocs := make(map[int32]*process.Process)
	var pids []int32
	pids, err := process.Pids()
	if err != nil {
		pids = make([]int32, 0)
	}
	for _, pid := range pids {
		p, err := process.NewProcessWithContext(ctx, pid)
		if err != nil {
			continue
		}
		oldprocs[pid] = p
	}
	for {
		tskick := <-tsCh
		time.Sleep(time.Until(tskick))
		ts := tskick.Add(-interval)
		pids, err := process.Pids()
		if err != nil {
			pids = make([]int32, 0)
		}
		for _, pid := range pids {
			isold := true
			var p *process.Process
			var ok bool
			if p, ok = oldprocs[pid]; !ok {
				isold = false
				p, err = process.NewProcessWithContext(ctx, pid)
				if err != nil {
					continue
				}
			}
			cmdline, err := p.Cmdline()
			if err != nil {
				if isold {
					delete(oldprocs, pid)
					continue
				}
			}
			pname, err := p.Name()
			if err != nil {
				if isold {
					delete(oldprocs, pid)
					continue
				}
			}
			//fmt.Printf("pname: %v\n", pname)
			uids, err := p.Uids()
			if err != nil {
				if isold {
					delete(oldprocs, pid)
					continue
				}
			}
			//fmt.Printf("uids: %v\n", uids)
			ppid, err := p.Ppid()
			if err != nil {
				if isold {
					delete(oldprocs, pid)
					continue
				}
			}
			//fmt.Printf("ppid: %v\n", ppid)
			cpuTotal, cpuUser, cpuSystem, cpuIowait, err := p.PercentAllWithContext(ctx, 0)
			if err != nil {
				if isold {
					delete(oldprocs, pid)
					continue
				}
			}
			//fmt.Printf("cpuTotal: %f\n", cpuTotal)
			//fmt.Printf("cpuUser: %f\n", cpuUser)
			//fmt.Printf("cpuSystem: %f\n", cpuSystem)
			//fmt.Printf("cpuIowait: %f\n", cpuIowait)
			num_thread, err := p.NumThreads()
			if err != nil {
				if isold {
					delete(oldprocs, pid)
					continue
				}
			}
			mem, err := p.MemoryInfo()
			if err != nil {
				if isold {
					delete(oldprocs, pid)
					continue
				}
			}
			newprocs[pid] = p
			const unitmem = 1024
			maxlen := optLenCmdline
			if maxlen < 0 || len(cmdline) < maxlen {
				maxlen = len(cmdline)
			}
			fmt.Printf("%s %s %5d %-20s %5d %5d %5.1f %6.0f %6.0f %6.0f %4d %10d %10d %s\n", ts.Format("2006-01-02T15:04:05"), phost, uids[0], pname, p.Pid, ppid, cpuTotal, cpuUser, cpuSystem, cpuIowait, num_thread, mem.VMS/unitmem, mem.RSS/unitmem, cmdline[:maxlen])

		}
		fmt.Printf("\n")
		oldprocs = newprocs
		newprocs = make(map[int32]*process.Process)
	}
}
