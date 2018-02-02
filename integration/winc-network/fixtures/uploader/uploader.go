package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	url := os.Args[1]

	num, err := strconv.Atoi(os.Args[2])
	if err != nil {
		panic(err)
	}

	data := make([]byte, num)
	buf := bytes.NewBuffer(data)
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		panic(err)
	}

	client := &http.Client{}
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	uploadDuration := time.Since(startTime) / time.Millisecond

	fmt.Printf("this is recieved in %d miliseconds\n", int64(uploadDuration))
}
