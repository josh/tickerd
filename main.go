package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/namsral/flag"
)

var (
	name      = "tickerd"
	envPrefix = "TICKERD"
)

var (
	fs              *flag.FlagSet
	intervalStr     string
	timeoutStr      string
	watchPath       string
	healthcheckFile string
	healthcheck     bool
)

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
	fs.StringVar(&intervalStr, "interval", "", "scheduling interval")
	fs.StringVar(&timeoutStr, "timeout", "", "command timeout")
	fs.StringVar(&watchPath, "watch", "", "watch path")
	fs.StringVar(&healthcheckFile, "healthcheck-file", "", "healthcheck file")
	fs.BoolVar(&healthcheck, "healthcheck", false, "run healthcheck")
	fs.Parse(os.Args[1:])

	var interval = time.Duration(0)
	if intervalStr != "" {
		var err error
		interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			usage(err.Error())
		}
	}

	var timeout = time.Duration(0)
	if timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			usage(err.Error())
		}
	}

	// -healthcheck requires -healthcheck-file
	if healthcheck && healthcheckFile == "" {
		usage()
	}

	// run docker healthcheck
	if healthcheck {
		_, err := os.Stat(healthcheckFile)
		if os.IsNotExist(err) {
			os.Exit(0)
		} else {
			os.Exit(1)
		}

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

	// run for first time
	run(args, timeout)

	// exit if only running once
	if interval.Seconds() == 0 {
		os.Exit(0)
	}

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)

	sigRun := make(chan os.Signal, 1)
	signal.Notify(sigRun, syscall.SIGUSR1)

	ticker := time.NewTicker(interval)

	// enable fsnotify on watch path
	watchChan := make(chan bool)
	if watchPath != "" {
		err := watch(watchChan, watchPath)
		if err != nil {
			usage(err.Error())
		}
	}

	for {
		select {
		case <-sigTerm:
			os.Exit(1)
		case <-watchChan:
			run(args, timeout)
		case <-sigRun:
			run(args, timeout)
		case <-ticker.C:
			run(args, timeout)
		}
	}
}

func run(args []string, timeout time.Duration) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println()
	fmt.Println("#", time.Now().Format(time.ANSIC))
	fmt.Println("+", strings.Join(args, " "))

	var timeoutTimer *time.Timer
	if timeout.Seconds() != 0 {
		timeoutTimer = time.AfterFunc(timeout, func() {
			cmd.Process.Kill()
		})
	}

	err := cmd.Run()

	if timeoutTimer != nil {
		timeoutTimer.Stop()
	}

	if err != nil {
		fmt.Println(err)
	}

	if healthcheckFile != "" {
		if err != nil {
			data := []byte(err.Error())
			ioutil.WriteFile(healthcheckFile, data, 0644)
		} else {
			os.Remove(healthcheckFile)
		}
	}
}

func watch(ch chan<- bool, name string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// TODO: watcher event not working
				fmt.Println("debug watch/event", event)
				ch <- true
			}
		}
	}()

	err = watcher.Add(name)
	if err != nil {
		return err
	}

	return nil
}
