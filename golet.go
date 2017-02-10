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
	"syscall"
	"time"

	"github.com/Code-Hex/golet/internal/port"
	colorable "github.com/mattn/go-colorable"
	"github.com/robfig/cron"
	"golang.org/x/sync/errgroup"
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
	// Proclet is great module!!
	config struct {
		interval   time.Duration // interval in seconds between spawning services unless a service exits abnormally.
		color      bool          // colored log.
		logger     io.Writer     // sets the output destination file. use stderr by default.
		logWorker  bool          // enable worker for format logs. If disabled this option, cannot use logger opt too.
		execNotice bool          // enable start and exec notice message like: `16:38:12 worker.1 | Start callback: worker``.

		services   []Service
		g          *errgroup.Group
		ctx        context.Context
		serviceNum int
		tags       map[string]bool
		cron       *cron.Cron
	}

	// Service struct to add services to golet.
	Service struct {
		Exec   string
		Code   func(io.Writer, int) // Routine of services.
		Worker int                  // Number of goroutine. The maximum number of workers is 100.
		Tag    string               // Keyword for log.
		Every  string               // Crontab like format. See https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format

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
	g, ctx := errgroup.WithContext(c)
	return &config{
		interval:   0,
		color:      false,
		logger:     colorable.NewColorableStderr(),
		logWorker:  true,
		execNotice: true,

		g:    g,
		ctx:  ctx,
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

	// Calculate of the number of workers.
	cap := 0
	for _, service := range c.services {
		cap += service.Worker
	}

	order := make([]string, 0, cap)

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
		if service.Code == nil && service.Exec != "" {
			cmd := service.prepare()
			// Notify you have executed the command
			if c.execNotice {
				fmt.Fprintf(service.pipe.writer, "Exec command: %s\n", service.Exec)
			}

			// Execute the command with cron or goroutine
			if service.Every != "" {
				c.addCmd(service, chps)
			} else {
				c.g.Go(func() error {
					defer service.pipe.writer.Close()
					return run(cmd, chps)
				})
			}
		}

		if service.Code != nil {
			// Notify you have run the callback
			if c.execNotice {
				fmt.Fprintf(service.pipe.writer, "Callback: %s\n", service.Tag)
			}

			// Run callback with cron or goroutine
			if service.Every != "" {
				c.addTask(service)
			} else {
				c.g.Go(func() error {
					defer service.pipe.writer.Close()
					go service.Code(service.pipe.writer, service.port)
					<-c.ctx.Done()
					return nil
				})
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

	return c.wait(chps, signals)
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
				sendSignal(syscall.SIGTERM, procs)
			case syscall.SIGINT:
				sendSignal(syscall.SIGINT, procs)
			}
		case <-c.ctx.Done():
			c.cron.Stop()
			return
		}
	}
}

// sendSignal can send signal and replace os.Process struct of the terminated process with nil
func sendSignal(sig syscall.Signal, procs []*os.Process) {
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
	c.cron.AddFunc(s.Every, func() {
		run(s.prepare(), chps)
	})
}

// Add a task to execute the code block to cron.
func (c *config) addTask(s Service) {
	c.cron.AddFunc(s.Every, func() {
		go s.Code(s.pipe.writer, s.port)
		<-c.ctx.Done()
	})
}

// Wait services
func (c *config) wait(chps chan<- *os.Process, sig chan<- os.Signal) error {
	c.cron.Start()
	err := c.g.Wait()
	signal.Stop(sig)
	return err
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
