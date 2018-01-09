package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
