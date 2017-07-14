package main

import (
	"fmt"
	"os"
)

func main() {
	stdin := make([]byte, 1024, 1024)
	_, err := os.Stdin.Read(stdin)
	if err != nil {
		fmt.Println("unable to read stdin: " + err.Error())
		os.Exit(1)
	}

	fmt.Printf("%s", stdin)
}
