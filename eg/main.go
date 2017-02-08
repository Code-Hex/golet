package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Code-Hex/golet"
)

func main() {
	p := golet.New(context.Background())
	p.EnableColor()
	// Execcution interval
	p.SetInterval(time.Second * 1)

	p.Env(map[string]string{
		"NAME":  "codehex",
		"VALUE": "121",
	})

	p.Add(
		golet.Service{
			Exec: "ping google.com",
			Tag:  "ping",
		},
		golet.Service{
			Exec:   "echo 'Worker is 2!! PORT: $PORT'",
			Every:  "30 * * * * *",
			Worker: 2,
			Tag:    "echo",
		},
	)

	p.Add(golet.Service{
		Code: func(w io.Writer, port int) error {
			fmt.Fprintln(w, "Hello golet!! Port:", port)
			fmt.Fprintln(w, os.Getenv("NAME"), os.Getenv("VALUE"))
			return nil
		},
		Worker: 3,
		Every:  "@every 10s",
	})

	p.Run()
}
