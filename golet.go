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
		basePort   int
		cron       *cron.Cron
	}

	// Service struct to add services to golet.
	Service struct {
		Exec   string
		Code   func(io.Writer) error // Routine of services.
		Worker int                   // Number of goroutine. The maximum number of workers is 100.
		Tag    string                // Keyword for log.
		Every  string                // Crontab like format.

		startPort int
		color     color
		pipe      pipe
	}

	pipe struct {
		reader *os.File
		writer *os.File
	}
)

// Runner interface have methods for configuration and to run services.
type Runner interface {
	SetInterval(time.Duration)
	EnableColor()
	SetLogger(io.Writer)
	DisableLogger()
	DisableExecNotice()
	Add(...Service) error
	Run() error
}

var (
	shell []string
	mu    = new(sync.Mutex)
)

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
			panic("Could not find `sh` command")
		}
		shell = []string{path, "-c"}
	}
}

// for setting
func (c *config) SetInterval(t time.Duration) { c.interval = t }
func (c *config) EnableColor()                { c.color = true }
func (c *config) SetLogger(f io.Writer)       { c.logger = f }
func (c *config) DisableLogger()              { c.logWorker = false }
func (c *config) DisableExecNotice()          { c.execNotice = false }

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

func (c *config) Env(envs map[string]string) error {
	for k := range envs {
		if e := os.Setenv(k, envs[k]); e != nil {
			return e
		}
	}
	return nil
}

func (c *config) Add(services ...Service) error {
	mu.Lock()
	defer mu.Unlock()

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

		service.startPort = n
		service.color = color(c.serviceNum%colornum + 32)

		c.services = append(c.services, service)
	}
	return nil
}

func (c *config) Run() error {
	services := make(map[string]Service)

	// calculate of the number of workers
	cap := 0
	for _, service := range c.services {
		cap += service.Worker
	}

	order := make([]string, 0, cap)

	// assignment services
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
			s.startPort += i
			services[sid] = s
			order = append(order, sid)
		}
	}

	chps := make(chan *os.Process)

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

	go c.waitSignals(signals, chps, len(order))

	// invoke workers
	for _, sid := range order {
		service := services[sid]
		if service.Code == nil && service.Exec != "" {
			cmd := service.prepare()
			if service.Every != "" {
				c.cron.AddFunc(service.Every, func() {
					run(cmd, chps)
				})
			} else {
				c.g.Go(func() error {
					defer service.pipe.writer.Close()
					return run(cmd, chps)
				})
			}
		}

		if service.Code != nil {
			if service.Every != "" {
				c.cron.AddFunc(service.Every, func() {
					service.Code(service.pipe.writer)
				})
			} else {
				c.g.Go(func() error {
					defer service.pipe.writer.Close()
					return service.Code(service.pipe.writer)
				})
			}
		}

		// enable log worker if logWorker is true
		if c.logWorker && (service.Code != nil || service.Exec != "") {
			rd := service.pipe.reader
			go c.logging(bufio.NewScanner(rd), sid, service.color)
		}
		time.Sleep(c.interval)
	}

	c.cron.Start()

	return c.g.Wait()
}

func (c *config) waitSignals(signals <-chan os.Signal, chps <-chan *os.Process, cap int) {
	procs := make([]*os.Process, 0, cap)
	for {
		select {
		case proc := <-chps:
			procs = append(procs, proc)
		case s := <-signals:
			switch s {
			case syscall.SIGTERM, syscall.SIGHUP:
				for _, p := range procs {
					p.Signal(syscall.SIGTERM)
				}
			case syscall.SIGINT:
				for _, p := range procs {
					p.Signal(syscall.SIGINT)
				}
			}
		case <-c.ctx.Done():
			c.cron.Stop()
			return
		}
	}
}

func run(c *exec.Cmd, chps chan<- *os.Process) error {
	if err := c.Start(); err != nil {
		return err
	}
	chps <- c.Process
	return c.Wait()
}

func (s *Service) prepare() *exec.Cmd {
	args := append(shell, s.Exec)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = s.pipe.writer
	cmd.Stderr = s.pipe.writer
	return cmd
}

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
