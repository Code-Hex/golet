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
	"strings"
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

type (
	// config is main struct.
	// struct comments from http://search.cpan.org/dist/Proclet/lib/Proclet.pm
	// Proclet is a great module!!
	config struct {
		interval   time.Duration // interval in seconds between spawning services unless a service exits abnormally.
		color      bool          // colored log.
		logger     io.Writer     // sets the output destination file. use stderr by default.
		logWorker  bool          // enable worker for format logs. If disabled this option, cannot use logger opt too.
		execNotice bool          // enable start and exec notice message like: `16:38:12 worker.1 | Start callback: worker``.

		services   []Service
		wg         sync.WaitGroup
		once       sync.Once
		cancel     func()
		ctx        context.Context
		serviceNum int
		tags       map[string]bool
		cron       *cron.Cron
	}

	// Service struct to add services to golet.
	Service struct {
		Exec   string
		Code   func(context.Context, io.Writer, int) // Routine of services.
		Worker int                                   // Number of goroutine. The maximum number of workers is 100.
		Tag    string                                // Keyword for log.
		Every  string                                // Crontab like format. See https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format

		port  int
		color color
		pipe  pipe
	}

	pipe struct {
		reader *os.File
		writer *os.File
	}
)

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
	Env(map[string]string) error
	Add(...Service) error
	Run() error
}

// for settings
// SetInterval can specify the interval at which the command is executed.
func (c *config) SetInterval(t time.Duration) { c.interval = t }

// EnableColor can output colored log.
func (c *config) EnableColor() { c.color = true }

// SetLogger can specify the *os.File
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

// New to create struct of golet.
func New(c context.Context) Runner {
	ctx, cancel := context.WithCancel(c)
	return &config{
		interval:   0,
		color:      false,
		logger:     colorable.NewColorableStderr(),
		logWorker:  true,
		execNotice: true,

		ctx:    ctx,
		cancel: cancel,
		tags:   map[string]bool{},
		cron:   cron.New(),
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
		} else if service.Worker > 100 {
			service.Worker = 100
		}

		if _, ok := c.tags[service.Tag]; ok {
			return errors.New("tag: " + service.Tag + " is already exists")
		}
		c.tags[service.Tag] = true

		n, err := port.GetPort()
		if err != nil {
			return err
		}

		service.port = n
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
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

	go c.waitSignals(signals, chps, len(order))

	// Invoke workers.
	for _, sid := range order {
		service := services[sid]
		if service.isExecute() {
			// Execute the command with cron or goroutine
			if service.isCron() {
				c.addCmd(service, chps)
			} else {
				c.wg.Add(1)

				go func() {
					defer func() {
						service.pipe.writer.Close()
						c.wg.Done()
					}()

					for {
						// Notify you have executed the command
						if c.execNotice {
							fmt.Fprintf(service.pipe.writer, "Exec command: %s\n", service.Exec)
						}
						select {
						case <-c.ctx.Done():
							return
						default:
							run(service.prepare(), chps)
						}
					}
				}()
			}
		}

		if service.isCode() {
			// Run callback with cron or goroutine
			if service.isCron() {
				c.addTask(service)
			} else {
				c.wg.Add(1)

				go func() {
					defer func() {
						service.pipe.writer.Close()
						c.wg.Done()
					}()

					for {
						// Notify you have run the callback
						if c.execNotice {
							fmt.Fprintf(service.pipe.writer, "Callback: %s\n", service.Tag)
						}
						select {
						case <-c.ctx.Done():
							return
						default:
							service.Code(c.ctx, service.pipe.writer, service.port)
						}
					}
				}()
			}
		}

		// Enable log worker if logWorker is true.
		if c.logWorker && (service.Code != nil || service.Exec != "") {
			rd := service.pipe.reader
			go c.logging(bufio.NewScanner(rd), sid, service.color)
		}

		// When the task is cron, it does not cause wait time.
		if service.Every == "" {
			time.Sleep(c.interval)
		}
	}

	c.wait(chps, signals)

	return nil
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
			in, out, err := os.Pipe()
			if err != nil {
				return err
			}

			s := service
			sid := fmt.Sprintf("%s.%d", s.Tag, i)

			s.pipe = pipe{in, out}
			s.port += i
			services[sid] = s
			*order = append(*order, sid)
		}
	}

	return nil
}

// Receive process ID to be executed. or
// It traps the signal relate to parent process. sends a signal to the received process ID.
func (c *config) waitSignals(signals <-chan os.Signal, chps <-chan *os.Process, cap int) {
	procs := make([]*os.Process, 0, cap)
Loop:
	for {
		select {
		case proc := <-chps:
			// Replace the used process(nil) with the newly generated process.
			// This run to reduce the memory allocation frequency.
			for i, p := range procs {
				if p == nil {
					procs[i] = proc
					continue Loop
				}
			}
			// If not used all processes, allocated newly.
			procs = append(procs, proc)
		case s := <-signals:
			switch s {
			case syscall.SIGTERM, syscall.SIGHUP:
				c.once.Do(func() {
					if c.cancel != nil {
						c.cancel()
					}
					time.Sleep(time.Second * 1)
				})

				sendSignal2Procs(syscall.SIGTERM, procs)
			case syscall.SIGINT:
				sendSignal2Procs(syscall.SIGINT, procs)
			}
		case <-c.ctx.Done():
			c.cron.Stop()
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

// Create a command
func (s *Service) prepare() *exec.Cmd {
	c := strings.Replace(s.Exec, "$PORT", fmt.Sprintf("%d", s.port), -1)
	args := append(shell, c)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = s.pipe.writer
	cmd.Stderr = s.pipe.writer
	return cmd
}

// Add a task to execute the command to cron.
func (c *config) addCmd(s Service, chps chan<- *os.Process) {
	// Notify you have executed the command
	if c.execNotice {
		fmt.Fprintf(s.pipe.writer, "Exec command: %s\n", s.Exec)
	}
	c.cron.AddFunc(s.Every, func() {
		run(s.prepare(), chps)
	})
}

// Add a task to execute the code block to cron.
func (c *config) addTask(s Service) {
	// Notify you have run the callback
	if c.execNotice {
		fmt.Fprintf(s.pipe.writer, "Callback: %s\n", s.Tag)
	}

	c.cron.AddFunc(s.Every, func() {
		s.Code(c.ctx, s.pipe.writer, s.port)
	})
}

// Wait services
func (c *config) wait(chps chan<- *os.Process, sig chan<- os.Signal) {
	c.cron.Start()
	c.wg.Wait()
	signal.Stop(sig)
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

func (s *Service) isExecute() bool {
	return s.Code == nil && s.Exec != ""
}

func (s *Service) isCode() bool {
	return s.Code != nil
}

func (s *Service) isCron() bool {
	return s.Every != ""
}
