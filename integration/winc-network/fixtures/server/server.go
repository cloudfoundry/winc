package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
	http.HandleFunc("/download", downloadHandler)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: nil,
	}

	log.Fatal(server.ListenAndServe())
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	_, err := io.Copy(io.Discard, r.Body)
	if err != nil {
		panic(err)
	}
}
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	size, err := strconv.Atoi(r.URL.Query().Get("size"))
	if err != nil {
		panic(err)
	}

	fmt.Printf("server recieved size: %d\n", size)

	data := make([]byte, size)
	_, err = w.Write(data)
	if err != nil {
		panic(err)
	}
}
