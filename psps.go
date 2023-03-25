package main

import (
        _ "embed"
	"os"
	"fmt"
	"flag"
	"path/filepath"
	"time"
	//"context"
	//"github.com/shirou/gopsutil/v3/cpu"
	//"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/shirou/gopsutil/v3/host"
	// "github.com/shirou/gopsutil/mem"  // to use v2
)
var (
        //go:embed version.txt
        version string

        initCfg     bool
        showVersion bool
        optInterval  int
        cfgFile     string
)


func main() {
        flag.BoolVar(&showVersion, "v", false, "show version information")
        flag.BoolVar(&initCfg, "init", false, "create default cfgfile if it does not exist")
        flag.IntVar(&optInterval, "i", 10, "interval sec")
        flag.StringVar(&cfgFile, "f", "./cfg.yml", "config file")
        flag.Parse()

        if showVersion {
                f1, _ := filepath.Abs(".")
                f2, _ := filepath.Abs("logs")
                fmt.Printf("%s\n%s\n%s\n", version, f1, f2)
                os.Exit(0)
        }


	phost, err := host.Info()
	if err != nil {
		panic(err)
	}
	interval := time.Duration(optInterval) * time.Second
	fmt.Printf("time hostname Uid comm pid ppid cpu usr sys iowait num_threads VmSize VmRSS cmdline\n")
	tsCh := make(chan time.Time)
	go pacemaker(tsCh, interval)
	for {
		ts := <- tsCh
		time.Sleep(time.Until(ts))
		sub1(phost.Hostname, ts.Add(-interval))
	}
}

func pacemaker(tsCh chan time.Time, interval time.Duration ) {
	startTime := time.Now().Add(time.Second)
	currentTime := startTime
	for {
		tsCh <- currentTime.Add(interval)
		currentTime = currentTime.Add(interval)
	}
}
func sub1(phost string, ts time.Time) {
	//v, _ := mem.VirtualMemory()

	// almost every return value is a struct
	//fmt.Printf("Total: %v, Free:%v, UsedPercent:%f%%\n", v.Total, v.Free, v.UsedPercent)

	// convert to JSON. String() is also implemented
	//ctx := context.Background()
	//fmt.Println(v)
	//c, _ := cpu.PercentWithContext(ctx, -1, true) 
	//fmt.Printf("cpu_percent: %v\n", c)
	ps, err := process.Processes()
	if err != nil {
		panic(err)
	}
	for _,p := range ps {
		cmdline, err := p.Cmdline()
                if err != nil {
                        panic(err)
                }
		pname ,err := p.Name()
		if err != nil {
			panic(err)
		}
		//fmt.Printf("pname: %v\n", pname)
		uids ,err := p.Uids()
		if err != nil {
			panic(err)
		}
		//fmt.Printf("uids: %v\n", uids)
		ppid ,err := p.Ppid()
		if err != nil {
			panic(err)
		}
		//fmt.Printf("ppid: %v\n", ppid)
		cpuTotal, cpuUser, cpuSystem, cpuIowait, err := p.PercentAll(0)
		if err != nil {
			panic(err)
		}
		//fmt.Printf("cpuTotal: %f\n", cpuTotal)
		//fmt.Printf("cpuUser: %f\n", cpuUser)
		//fmt.Printf("cpuSystem: %f\n", cpuSystem)
		//fmt.Printf("cpuIowait: %f\n", cpuIowait)
		num_thread ,err := p.NumThreads()
		if err != nil {
			panic(err)
		}
		mem, err := p.MemoryInfo()
                if err != nil {
                        panic(err)
                }

		//proc, err := process.NewProcess(p)
		//if err != nil {
		//	fmt.Printf("ERROR: %v\n", err)
		//}
		//fmt.Printf("%v\n",proc)
		fmt.Printf("%s %s %5d %-36s %5d %5d %6.1f %6.1f %6.1f %6.1f, %4d %10d %10d %s\n",ts.Format("2006-01-02T15:04:05"), phost, uids[0], pname, p.Pid, ppid, cpuTotal,cpuUser,cpuSystem,cpuIowait, num_thread, mem.VMS, mem.RSS, cmdline)
	}
	fmt.Printf("\n")

}
