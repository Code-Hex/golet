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
	/*
		if err := agent.Listen(nil); err != nil {
			log.Fatal(err)
		}
	*/
	p := golet.New(context.Background())
	p.EnableColor()
	p.SetInterval(time.Second * 0)

	p.Env(map[string]string{
		"NAME":  "KEI",
		"VALUE": "121",
	})

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
			fmt.Println(os.Getenv("NAME"), os.Getenv("VALUE"))
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
