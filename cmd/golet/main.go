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
	p.SetInterval(time.Second * 1)

	p.Env(map[string]string{
		"NAME":  "KEI",
		"VALUE": "121",
	})

	p.Add(
		golet.Service{
			Exec: []string{"echo", "Hello!!"},
			// Every when 30 seconds
			Every: "30 * * * * *",
		},
		golet.Service{
			Exec:   []string{"ping", "google.com"},
			Worker: 4,
		},
	)

	p.Add(golet.Service{
		Code: func(w io.Writer) error {
			fmt.Fprintln(w, "Hello golet!!")
			fmt.Println(os.Getenv("NAME"), os.Getenv("VALUE"))
			return nil
		},
		Worker: 4,
		Every:  "@every 10s",
	})

	p.Run()
}
