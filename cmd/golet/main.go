package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Code-Hex/golet"
)

func main() {
	/*
		if err := agent.Listen(nil); err != nil {
			log.Fatal(err)
		}
	*/
	p := golet.New(context.Background())
	p.EnableColor()
	p.SetInterval(time.Second * 0)

	p.Add(
		golet.Service{
			Exec: "ping google.com",
			//Every: "33 * * * * *",
		},
		golet.Service{
			Exec:   "ping ie.u-ryukyu.ac.jp",
			Worker: 2,
		},
	)

	p.Add(golet.Service{
		Code: func(w io.Writer) error {
			fmt.Fprintln(w, "Hello golet!!")
			return nil
		},
		Worker: 4,
		Every:  "33 * * * * *",
	})

	p.Add(golet.Service{
		Exec:   "ping gigazine.net",
		Worker: 1,
	})

	p.Run()
}
