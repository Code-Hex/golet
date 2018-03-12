package golet

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/Code-Hex/golet/internal/port"
	colorable "github.com/mattn/go-colorable"
	"github.com/robfig/cron"
)

type color int

const (
	red color = iota // + 31
	green
	yellow
	blue
	magenta
	cyan

	colornum int = 5
)

// config is main struct.
// struct comments from http://search.cpan.org/dist/Proclet/lib/Proclet.pm
// Proclet is a great module!!
type config struct {
	interval     time.Duration  // interval in seconds between spawning services unless a service exits abnormally.
	color        bool           // colored log.
	logger       io.Writer      // sets the output destination file. use stderr by default.
	logWorker    bool           // enable worker for format logs. If disabled this option, cannot use logger opt too.
	execNotice   bool           // enable start and exec notice message like: `16:38:12 worker.1 | Start callback: worker``.
	cancelSignal syscall.Signal // sets the syscall.Signal to notify context cancel. If you sets something, you can send that signal to processes when context cancel.

	services   []Service
	wg         sync.WaitGroup
	ctx        *signalCtx
	serviceNum int
	tags       map[string]bool
	cron       *cron.Cron
}

var shell []string

func init() {
	if runtime.GOOS == "windows" {
		path, err := exec.LookPath("cmd")
		if err != nil {
			panic("Could not find `cmd` command")
		}
		shell = []string{path, "/c"}
	} else {
		path, err := exec.LookPath("bash")
		if err != nil {
			panic("Could not find `bash` command")
		}
		shell = []string{path, "-c"}
	}
}

// Runner interface have methods for configuration and to run services.
type Runner interface {
	SetInterval(time.Duration)
	EnableColor()
	SetLogger(io.Writer)
	DisableLogger()
	DisableExecNotice()
	SetCtxCancelSignal(syscall.Signal)
	Env(map[string]string) error
	Add(...Service) error
	Run() error
}

// for settings
// SetInterval can specify the interval at which the command is executed.
func (c *config) SetInterval(t time.Duration) { c.interval = t }

// EnableColor can output colored log.
func (c *config) EnableColor() { c.color = true }

// SetLogger can specify the io.Writer
// for example in https://github.com/lestrrat/go-file-rotatelogs
/*
      logf, _ := rotatelogs.New(
  	    "/path/to/access_log.%Y%m%d%H%M",
  	    rotatelogs.WithLinkName("/path/to/access_log"),
  	    rotatelogs.WithMaxAge(24 * time.Hour),
  	    rotatelogs.WithRotationTime(time.Hour),
      )

	  golet.New(context.Background()).SetLogger(logf)
*/
func (c *config) SetLogger(f io.Writer) { c.logger = f }

// DisableLogger is prevent to output log
func (c *config) DisableLogger() { c.logWorker = false }

// DisableExecNotice is disable execute notifications
func (c *config) DisableExecNotice() { c.execNotice = false }

// SetCtxCancelSignal can specify the signal to send processes when context cancel.
func (c *config) SetCtxCancelSignal(signal syscall.Signal) { c.cancelSignal = signal }

// New to create struct of golet.
func New(ctx context.Context) Runner {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	return &config{
		interval:     0,
		color:        false,
		logger:       colorable.NewColorableStderr(),
		logWorker:    true,
		execNotice:   true,
		cancelSignal: -1, // -1 does not exist. see, https://golang.org/src/syscall/syscall_unix.go?s=3494:3525#L141

		ctx: &signalCtx{
			parent:  ctx,
			sigchan: signals,
		},
		tags: map[string]bool{},
		cron: cron.New(),
	}
}

// Env can add temporary environment variables.
func (c *config) Env(envs map[string]string) error {
	for k := range envs {
		if e := os.Setenv(k, envs[k]); e != nil {
			return e
		}
	}
	return nil
}

// Add can add runnable services
func (c *config) Add(services ...Service) error {
	for _, service := range services {
		c.serviceNum++
		if service.Tag == "" {
			service.Tag = fmt.Sprintf("%d", c.serviceNum)
		}
		if service.Worker <= 0 {
			service.Worker = 1
		}
		if _, ok := c.tags[service.Tag]; ok {
			return errors.New("tag: " + service.Tag + " is already exists")
		}
		c.tags[service.Tag] = true

		n, err := port.GetPort()
		if err != nil {
			return err
		}

		service.tmpPort = n
		service.color = color(c.serviceNum%colornum + 32)

		c.services = append(c.services, service)
	}
	return nil
}

// Run just like the name.
func (c *config) Run() error {
	services := make(map[string]Service)

	order := make([]string, 0, c.calcCapacitySize())

	// Assign services.
	if err := c.assign(&order, services); err != nil {
		return err
	}
	chps := make(chan *os.Process, 1)
	go c.waitSignals(chps, len(order))

	// Invoke workers.
	for _, sid := range order {
		service := services[sid]
		// Run one for each service.
		if service.isExecute() {
			c.executeRun(service, chps)
		} else if service.isCode() {
			c.executeCallback(service)
		}
		// Enable log worker if logWorker is true.
		if c.logWorker && (service.Code != nil || service.Exec != "") {
			rd := service.reader
			go c.logging(bufio.NewScanner(rd), sid, service.color)
		}
		// When the task is cron, it does not cause wait time.
		if service.Every == "" {
			time.Sleep(c.interval)
		}
	}

	c.wait(chps)

	return nil
}

// executeRun run command as a process
func (c *config) executeRun(service Service, chps chan<- *os.Process) {
	// Execute the command with cron or goroutine
	if service.isCron() {
		c.addCmd(service, chps)
	} else {
		c.wg.Add(1)
		go func() {
			defer func() {
				service.ctx.Close()
				c.wg.Done()
			}()
		PROCESS:
			for {
				// Notify you have executed the command
				if c.execNotice {
					service.Printf("Exec command: %s\n", service.Exec)
				}
				select {
				case <-c.ctx.Done():
					return
				default:
					// If golet is received signal or exit code is 0, golet do not restart process.
					if err := run(service.prepare(), chps); err != nil {
						if exiterr, ok := err.(*exec.ExitError); ok {
							// See https://stackoverflow.com/a/10385867
							if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
								if !status.Signaled() {
									continue PROCESS
								}
								return
							}
						}
					}
					return
				}
			}
		}()
	}
}

// executeCallback run callback as goroutine or cron
func (c *config) executeCallback(service Service) {
	// Run callback with cron or goroutine
	if service.isCron() {
		c.addTask(service)
	} else {
		c.wg.Add(1)
		go func() {
			defer func() {
				service.ctx.Close()
				c.wg.Done()
			}()
			// If this callback is dead, we should restart it. (like a supervisor)
			// So, this loop for that.
		CALLBACK:
			for {
				// Notify you have run the callback
				if c.execNotice {
					service.Printf("Callback: %s\n", service.Tag)
				}
				select {
				case <-c.ctx.Done():
					return
				default:
					if err := service.Code(service.ctx); err != nil {
						service.Printf("Callback Error: %s\n", err.Error())
						continue CALLBACK
					}
					return
				}
			}
		}()
	}
}

// Calculate of the number of workers.
func (c *config) calcCapacitySize() (cap int) {
	for _, service := range c.services {
		cap += service.Worker
	}
	return
}

// Assign the service ID.
// It also make `order` slice to keep key order of `map[string]Service`.
func (c *config) assign(order *[]string, services map[string]Service) error {
	for _, service := range c.services {
		worker := service.Worker
		for i := 1; i <= worker; i++ {
			s := service
			if err := s.createContext(c.ctx, i); err != nil {
				return err
			}
			sid := fmt.Sprintf("%s.%d", s.Tag, i)
			services[sid] = s
			*order = append(*order, sid)
		}
	}
	return nil
}

// Receive process ID to be executed. or
// It traps the signal relate to parent process. sends a signal to the received process ID.
func (c *config) waitSignals(chps <-chan *os.Process, cap int) {
	procs := make([]*os.Process, 0, cap)
Loop:
	for {
		select {
		case proc := <-chps:
			// Replace used process(nil) with the newly generated process.
			// This run to reduce the memory allocation frequency.
			for i, p := range procs {
				if p == nil {
					procs[i] = proc
					continue Loop
				}
			}
			// If using all processes, allocate newly.
			procs = append(procs, proc)
		case c.ctx.signal = <-c.ctx.sigchan:
			switch c.ctx.signal {
			case syscall.SIGTERM, syscall.SIGHUP:
				// Send signals to each process as SIGTERM.
				// However, In the case of Goroutine, it will notify the received signal. (SIGTERM or SIGHUP)
				sendSignal2Procs(syscall.SIGTERM, procs)
				c.ctx.notifySignal()
			case syscall.SIGINT:
				sendSignal2Procs(syscall.SIGINT, procs)
				c.ctx.notifySignal()
			}
		case <-c.ctx.Done():
			if 0 <= c.cancelSignal {
				sendSignal2Procs(c.cancelSignal, procs)
				c.ctx.notifySignal()
			}
			return
		}
	}
}

// sendSignal2Procs can send signal and replace os.Process struct of the terminated process with nil
func sendSignal2Procs(sig syscall.Signal, procs []*os.Process) {
	for i, p := range procs {
		if p != nil {
			p.Signal(sig)
			// In case of error, the process has already finished.
			if _, err := p.Wait(); err != nil {
				procs[i] = nil
			}
		}
	}
}

// Execute the command and send its process ID.
func run(c *exec.Cmd, chps chan<- *os.Process) error {
	if err := c.Start(); err != nil {
		return err
	}
	chps <- c.Process
	return c.Wait()
}

// Add a task to execute the command to cron.
func (c *config) addCmd(s Service, chps chan<- *os.Process) {
	// Notify you have executed the command
	if c.execNotice {
		s.Printf("Exec command: %s\n", s.Exec)
	}
	c.cron.AddFunc(s.Every, func() {
		run(s.prepare(), chps)
	})
}

// Add a task to execute the code block to cron.
func (c *config) addTask(s Service) {
	// Notify you have run the callback
	if c.execNotice {
		s.Printf("Callback: %s\n", s.Tag)
	}
	c.cron.AddFunc(s.Every, func() {
		if err := s.Code(s.ctx); err != nil {
			s.Printf("Callback Error: %s\n", err.Error())
		}
	})
}

// Wait services
func (c *config) wait(chps chan<- *os.Process) {
	c.cron.Start()
	c.wg.Wait()
	c.cron.Stop()
	signal.Stop(c.ctx.sigchan)
}

// Logging
func (c *config) logging(sc *bufio.Scanner, sid string, clr color) {
	for sc.Scan() {
		hour, min, sec := time.Now().Clock()
		if c.color {
			fmt.Fprintf(c.logger, "\x1b[%dm%02d:%02d:%02d %-10s |\x1b[0m %s\n",
				clr,
				hour, min, sec, sid,
				sc.Text(),
			)
		} else {
			fmt.Fprintf(c.logger, "%02d:%02d:%02d %-10s | %s\n", hour, min, sec, sid, sc.Text())
		}
	}
}
