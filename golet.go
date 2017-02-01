package golet

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/Code-Hex/golet/internal/port"
	colorable "github.com/mattn/go-colorable"
)

type color int

const (
	red color = iota // + 31
	green
	yellow
	blue
	magenta
	cyan
)

const colornum = 5

type (
	// config is main struct
	// struct comments from http://search.cpan.org/dist/Proclet/lib/Proclet.pm
	// Proclet is great library!!
	config struct {
		interval   time.Duration // interval in seconds between spawning services unless a service exits abnormally
		color      bool          // colored log
		logger     io.Writer     // sets the output destination file. use stderr by default.
		logWorker  bool          // enable worker for format logs. If disabled this option, cannot use logger opt too.
		execNotice bool          // enable start and exec notice message like: `16:38:12 worker.1 | Start callback: worker``

		services   []Service
		serviceNum int
		tags       map[string]bool
		basePort   int
		m          *sync.Mutex
	}

	// Service struct to add services to golet
	Service struct {
		Exec   string
		Code   func() // Routine of services
		Worker int    // Number of goroutine. The maximum number of workers is 100.
		Tag    string // Keyword for log.
		Every  string // Crontab like format.

		color     int
		startPort int
		pipe      pipe
	}

	pipe struct {
		reader *os.File
		writer *os.File
	}
)

type Runner interface {
	SetInterval(time.Duration)
	EnableColor()
	SetLogger(io.Writer)
	DisableLogger()
	DisableExecNotice()
	Add(...Service)
	Run()
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

// New to create struct of golet
func New() Runner {
	return &config{
		interval:   0,
		color:      false,
		logger:     colorable.NewColorableStderr(),
		logWorker:  true,
		execNotice: true,

		tags: map[string]bool{},
	}
}

func (p *config) Add(services ...Service) {
	mu.Lock()
	defer mu.Unlock()

	for _, service := range services {
		p.serviceNum++

		if service.Tag == "" {
			service.Tag = fmt.Sprintf("%d", p.serviceNum)
		}

		if service.Worker <= 0 {
			service.Worker = 1
		}

		if _, ok := p.tags[service.Tag]; ok {
			die("tag: %s is already exists", service.Tag)
		}
		p.tags[service.Tag] = true

		n, err := port.GetPort()
		if err != nil {
			panic(err)
		}

		service.startPort = n
		service.color = p.serviceNum % colornum

		p.services = append(p.services, service)
	}
}

func (p *config) Run() {
	services := make(map[string]Service)

	// calculate of the number of workers
	cap := 0
	for _, service := range p.services {
		cap += service.Worker
	}

	order := make([]string, 0, cap)

	// assignment services
	for _, service := range p.services {
		worker := service.Worker
		for i := 1; i <= worker; i++ {
			in, out, err := os.Pipe()
			if err != nil {
				panic(err)
			}

			s := service
			sid := fmt.Sprintf("%s.%d", s.Tag, i)

			s.pipe.reader = in
			s.pipe.writer = out
			s.startPort += i
			services[sid] = s
			order = append(order, sid)
		}
	}

	var wg sync.WaitGroup

	// enable log worker if logWorker is true
	if p.logWorker {
		wg.Add(len(order) + 1)
		go func() {
			defer wg.Done()
			p.logging(services)
		}()
	} else {
		wg.Add(len(order))
	}

	// invoke workers
	for _, sid := range order {
		service := services[sid]
		if service.Code == nil && len(service.Exec) > 0 {
			go func(s Service) {
				defer wg.Done()
				s.exec()
			}(service)
		}
		time.Sleep(p.interval)
	}
	wg.Wait()
}

func (s *Service) exec() {
	args := append(shell, s.Exec)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdout = s.pipe.writer
	cmd.Stderr = s.pipe.writer
	cmd.Run()
}

func (p *config) logging(services map[string]Service) {
	readers := []*os.File{}
	fileno2sid := map[int]string{}

	for sid, s := range services {
		reader := s.pipe.reader
		fileno2sid[int(reader.Fd())] = sid
		readers = append(readers, reader)
	}

	for _, reader := range readers {
		go func(sc *bufio.Scanner, sid string) {
			for sc.Scan() {
				now := time.Now()
				hour, min, sec := now.Clock()
				if p.color {
					fmt.Fprintf(p.logger, "\x1b[%dm%02d:%02d:%02d %-10s |\x1b[0m %s\n",
						services[sid].color+32,
						hour, min, sec, sid,
						sc.Text(),
					)
				} else {
					fmt.Fprintf(p.logger, "%02d:%02d:%02d %-10s | %s\n", hour, min, sec, sid, sc.Text())
				}
			}
		}(bufio.NewScanner(reader), fileno2sid[int(reader.Fd())])
	}

}

func die(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}
