package golet

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

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
		assert.Equal(t, p1s[i].color, p2s[i].color)
		if 0 < i && i < ln-1 {
			assert.Equal(t, p1s[i-1].port+100, p1s[i].port)
			assert.Equal(t, p2s[i-1].port+100, p2s[i].port)
		}
	}
}

func TestAssign(t *testing.T) {
	st := ServiceGen()

	p := New(ctx)
	p.Add(st...)

	services := make(map[string]Service)

	// Calculate of the number of workers.
	cap := 0
	for _, service := range p.(*config).services {
		cap += service.Worker
	}

	order := make([]string, 0, cap)

	// Assignment services.
	if err := p.(*config).assign(&order, services); err != nil {
		t.Fatalf(err.Error())
	}

	assert.Equal(t, cap, len(order))

	x := 0
	for _, service := range p.(*config).services {
		worker := service.Worker
		for i := 1; i <= worker; i++ {
			s := service
			sid := fmt.Sprintf("%s.%d", s.Tag, i)
			assert.Equal(t, sid, order[x])
			x++
		}
	}
}

func ServiceGen() []Service {
	return []Service{
		{
			Exec: "ping google.com",
			Tag:  "ping",
		},
		{
			Code: func(w io.Writer, port int) error {
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
			Code: func(w io.Writer, port int) error {
				return nil
			},
			Every:  "@every 20s",
			Worker: -100,
			Tag:    "code-cron",
		},
		{
			Exec: "ping google.com",
			Code: func(w io.Writer, port int) error {
				return nil
			},
			Every:  "30 * * * * *",
			Worker: 40000,
			Tag:    "complex",
		},
	}
}
