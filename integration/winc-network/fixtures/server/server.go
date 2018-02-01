package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <port>", os.Args[0])
		os.Exit(1)
	}
	port := os.Args[1]

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Response from server on port %s", port)
	})
	http.HandleFunc("/upload", uploadHandler)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	n, err := io.Copy(ioutil.Discard, r.Body)
	if err != nil {
		panic(err)
	}
	uploadDuration := time.Now().Sub(startTime) / time.Millisecond

	fmt.Fprintf(w, "%d bytes are recieved in %d miliseconds", n, int64(uploadDuration))
}
