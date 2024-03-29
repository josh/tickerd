package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/namsral/flag"
	"github.com/robfig/cron/v3"
)

const (
	name      = "tickerd"
	envPrefix = "TICKERD"
	version   = "0.7.1"
)

var (
	fs                 *flag.FlagSet
	skipInitial        bool
	intervalStr        string
	cronStr            string
	timeoutStr         string
	watchPath          string
	healthcheckPort    int
	healthcheckAddress string
	healthcheck        bool
	printVersion       bool
)

var runErr error

func usage(message ...string) {
	if len(message) == 0 {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", name)
		fs.PrintDefaults()
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", message[0])
	}
	os.Exit(2)

}

func main() {
	fs = flag.NewFlagSetWithEnvPrefix(name, envPrefix, flag.ExitOnError)
	fs.BoolVar(&skipInitial, "skip-initial", false, "skip initial run")
	fs.StringVar(&intervalStr, "interval", "", "scheduling interval")
	fs.StringVar(&cronStr, "cron", "", "cron schedule")
	fs.StringVar(&timeoutStr, "timeout", "", "command timeout")
	fs.StringVar(&watchPath, "watch", "", "watch path")
	fs.IntVar(&healthcheckPort, "healthcheck-port", 0, "healthcheck port")
	fs.BoolVar(&healthcheck, "healthcheck", false, "run healthcheck")
	fs.BoolVar(&printVersion, "version", false, "print version")
	fs.Parse(os.Args[1:])

	if printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	var interval = time.Duration(0)
	if intervalStr != "" {
		var err error
		interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			usage(err.Error())
		}
	}

	var cronSchedule cron.Schedule
	if cronStr != "" {
		var err error
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		cronSchedule, err = parser.Parse(cronStr)
		if err != nil {
			usage(err.Error())
		}
		skipInitial = true
	}

	var timeout = time.Duration(0)
	if timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			usage(err.Error())
		}
	}

	if healthcheckPort != 0 {
		healthcheckAddress = ":" + strconv.Itoa(healthcheckPort)
	}

	// run docker healthcheck
	if healthcheck {
		conn, err := net.Dial("tcp", healthcheckAddress)
		if err != nil {
			log.Fatal(err)
		}
		buf := make([]byte, 1)
		buf[0] = 1
		conn.Read(buf)
		os.Exit(int(buf[0]))
	}

	// start healthcheck server
	if healthcheckAddress != "" {
		go healthcheckListen()
	}

	// handle "--"
	args := fs.Args()
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	// check for command
	if len(args) < 1 {
		usage()
	}

	// initial run
	if skipInitial == false {
		run(args, timeout)
	}

	// enable fsnotify on watch path
	var err error
	var watcher *fsnotify.Watcher
	watchChan := make(chan bool)
	if watchPath != "" {
		watcher, err = watch(watchChan, watchPath)
		if err != nil {
			usage(err.Error())
		}
		defer watcher.Close()
	}

	cronChan := make(chan bool)
	if cronSchedule != nil {
		c := cron.New()
		job := cron.FuncJob(func() { cronChan <- true })
		c.Schedule(cronSchedule, job)
		c.Start()
	}

	var ticker *time.Ticker
	var tickerChan <-chan time.Time
	if interval.Seconds() != 0 {
		// create real ticker chan if '-interval' defined
		ticker = time.NewTicker(interval)
		tickerChan = ticker.C
	}

	if ticker == nil && cronSchedule == nil && watcher == nil {
		// exit if only running once
		os.Exit(0)
	}

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)

	sigRun := make(chan os.Signal, 1)
	signal.Notify(sigRun, syscall.SIGUSR1)

	for {
		select {
		case <-sigTerm:
			os.Exit(1)
		case <-sigRun:
			run(args, timeout)
		case <-tickerChan:
			run(args, timeout)
		case <-cronChan:
			run(args, timeout)
		case <-watchChan:
			run(args, timeout)
		}
	}
}

func run(args []string, timeout time.Duration) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	fmt.Println()
	fmt.Println("#", time.Now().Format(time.ANSIC))
	fmt.Println("+", strings.Join(args, " "))

	var timeoutTimer *time.Timer
	if timeout.Seconds() != 0 {
		timeoutTimer = time.AfterFunc(timeout, func() {
			syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
			<-time.NewTimer(30 * time.Second).C
			syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
		})
	}

	runErr = cmd.Run()
	defer killProcessGroup(cmd.Process.Pid)

	if timeoutTimer != nil {
		timeoutTimer.Stop()
	}

	if runErr != nil {
		fmt.Println(runErr)
	}
}

func killProcessGroup(pgid int) {
	termTimeout := time.NewTimer(5 * time.Second)
	killTimeout := time.NewTimer(30 * time.Second)

	noChildren := make(chan bool, 1)
	go waitProcessGroup(pgid, noChildren)

	for {
		select {
		case <-noChildren:
			return
		case <-termTimeout.C:
			syscall.Kill(-pgid, syscall.SIGTERM)
		case <-killTimeout.C:
			syscall.Kill(-pgid, syscall.SIGKILL)
		}
	}
}

func waitProcessGroup(pgid int, done chan<- bool) {
	for {
		var wstatus syscall.WaitStatus
		_, err := syscall.Wait4(-pgid, &wstatus, 0, nil)

		if syscall.ECHILD == err {
			done <- true
			break
		}
	}
}

func watch(ch chan<- bool, name string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			<-watcher.Events
			ch <- true
		}
	}()

	err = watcher.Add(name)
	if err != nil {
		return nil, err
	}

	return watcher, nil
}

func healthcheckListen() {
	listener, err := net.Listen("tcp", healthcheckAddress)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		if runErr == nil {
			conn.Write([]byte{0})
		} else {
			conn.Write([]byte{1})
		}
		conn.Close()
	}
}
