// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package golet

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	colorable "github.com/mattn/go-colorable"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func TestDefault(t *testing.T) {
	p := New(ctx)

	assert.Equal(t, time.Duration(0), p.(*config).interval)
	assert.Equal(t, false, p.(*config).color)
	assert.Equal(t, colorable.NewColorableStderr(), p.(*config).logger)
	assert.Equal(t, true, p.(*config).logWorker)
	assert.Equal(t, true, p.(*config).execNotice)
}

func TestEnv(t *testing.T) {
	maps := []map[string]string{
		{"Hello": "World"},
		{
			"Year":  "2017",
			"Month": "02",
			"Day":   "08",
		},
	}

	for _, m := range maps {
		New(ctx).Env(m)
		for k := range m {
			env := os.Getenv(k)
			if m[k] != env {
				assert.Equal(t, m[k], env)
			}
		}
	}
}

func TestAdd(t *testing.T) {
	st := ServiceGen()

	p1, p2 := New(ctx), New(ctx)
	for _, v := range st {
		if err := p1.Add(v); err != nil {
			t.Fatalf(err.Error())
		}
	}
	if err := p2.Add(st...); err != nil {
		t.Fatalf(err.Error())
	}

	p1s := p1.(*config).services
	p2s := p2.(*config).services

	assert.Equal(t, len(p1s), len(p2s))

	ln := len(p1s)
	for i := 0; i < ln; i++ {
		assert.Equal(t, p1s[i].Exec, p2s[i].Exec)
		assert.Equal(t, p1s[i].Tag, p2s[i].Tag)
		assert.Equal(t, p1s[i].Every, p2s[i].Every)
		assert.Equal(t, p1s[i].Worker, p2s[i].Worker)
	}
}

func TestWait(t *testing.T) {
	c := exec.Command("go", "build", "-o", "sleep", "sleep.go")
	c.Dir = "_testdata"
	defer os.Remove(filepath.Join(c.Dir, "sleep"))
	if err := c.Run(); err != nil {
		t.Fatalf(err.Error())
	}

	_ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	p := New(_ctx)

	chps := make(chan *os.Process, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

	times := 3
	go p.(*config).waitSignals(chps, times)

	finch := make(chan bool)
	for i := 0; i < times; i++ {
		go func() {
			cmd := exec.Command("./sleep", "5")
			cmd.Dir = "_testdata"
			if err := run(cmd, chps); err == nil {
				panic(err)
			}
			finch <- true
		}()
	}

	// Send a signal to self test
	go func() {
		time.Sleep(time.Second * 1)
		syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	}()

	i := 0
	for i < times {
		select {
		case <-finch:
			i++
		case <-_ctx.Done():
			t.Fatalf("Timeout: Could not send signals to all processes")
		}
	}

	p.(*config).wait(chps)

	assert.Equal(t, times, i, "Could not send signals to all processes")
}

func ServiceGen() []Service {
	return []Service{
		{
			Exec: "ping google.com",
			Tag:  "ping",
		},
		{
			Code: func(c context.Context) error {
				return nil
			},
			Tag: "code",
		},
		{
			Exec:   "ping google.com",
			Worker: 4,
			Tag:    "ping-worker",
		},
		{
			Exec:   "ping google.com",
			Every:  "@hourly",
			Worker: 10,
			Tag:    "ping-cron",
		},
		{
			Code: func(c context.Context) error {
				return nil
			},
			Every:  "@every 20s",
			Worker: -100,
			Tag:    "code-cron",
		},
	}
}
