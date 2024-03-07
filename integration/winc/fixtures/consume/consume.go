package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	mem, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("bad memory arg: " + err.Error())
		os.Exit(1)
	}

	sleepTime := 1
	if len(os.Args) > 2 {
		sleepTime, err = strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Println("bad sleep time arg: " + err.Error())
			os.Exit(1)
		}
	}

	a := make([]byte, mem)
	fmt.Printf("Allocated %d\n", len(a))

	time.Sleep(time.Duration(sleepTime) * time.Second)

	os.Exit(0)
}
