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
	}

	var sleepTime int
	if len(os.Args) > 2 {
		sleepTime, err = strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Println("bad sleep time arg: " + err.Error())
		}
	}

	a := make([]byte, mem, mem)
	fmt.Printf("Allocated %d\n", len(a))

	time.Sleep(time.Duration(sleepTime) * time.Second)

	os.Exit(0)
}
