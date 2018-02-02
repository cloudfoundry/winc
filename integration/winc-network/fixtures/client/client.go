package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	url := os.Args[1]

	num, err := strconv.Atoi(os.Args[3])
	if err != nil {
		panic(err)
	}

	switch os.Args[2] {
	case "upload":
		upload(url, num)
	case "download":
		download(url, num)
	default:
		panic(fmt.Sprintf("unknown request: %s", os.Args[2]))

	}

}

func upload(url string, num int) {
	data := make([]byte, num)
	buf := bytes.NewBuffer(data)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/upload", url), buf)
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

	fmt.Printf("uploaded in %d miliseconds\n", int64(uploadDuration))
}

func download(url string, num int) {
	// uri := fmt.Sprintf("%s/download?size=%d", url, num)
	// fmt.Println(uri)
	uri := url
	req, err := http.NewRequest("GET", uri, nil)
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
	_, err = io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		panic(err)
	}
	downloadDuration := time.Since(startTime) / time.Millisecond

	fmt.Printf("downloaded in %d miliseconds\n", int64(downloadDuration))
}
