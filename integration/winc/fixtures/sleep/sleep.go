package main

import (
	"os"
	"strconv"
	"time"
)

func main() {
	sleepSeconds := 99999
	if len(os.Args) > 1 {
		var err error
		sleepSeconds, err = strconv.Atoi(os.Args[1])
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(time.Duration(sleepSeconds) * time.Second)
}
