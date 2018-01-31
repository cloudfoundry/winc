package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
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
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(contents))
}
