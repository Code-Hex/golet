package main

import (
	"os"
	"strconv"
	"time"
)

func main() {
	args := os.Args[1:]
	var sec time.Duration = 1
	if len(args) > 0 {
		t, err := strconv.Atoi(args[0])
		if err != nil {
			panic(err)
		}
		sec = time.Duration(t)
	}
	time.Sleep(time.Second * sec)
}
