package main

import (
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"
)

/*
 * See https://docs.microsoft.com/en-us/windows/console/handlerroutine
 */
const CTRL_SHUTDOWN_EVENT uint = 0x6

func eventHandler(controlType uint) uint {
	fmt.Printf("consoleControlHandler called with %v\n", controlType)

	if controlType == CTRL_SHUTDOWN_EVENT {
		fmt.Printf("Received CTRL_SHUTDOWN_EVENT (%d)\n", CTRL_SHUTDOWN_EVENT)
		fmt.Printf("Looping forever\n")
		start := time.Now()
		for {
			fmt.Printf("IN LOOP: Elapsed time=%s\n", time.Since(start))
			time.Sleep(500 * time.Millisecond)
		}
	}
	return 0
}

func main() {
	fmt.Println("Starting goshut")
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	setConsoleCtrlHandler := kernel32.NewProc("SetConsoleCtrlHandler")

	/*
	 * Calls the systemcall to set a ConsoleHandler
	 */
	r1, r2, lastErr := setConsoleCtrlHandler.Call(syscall.NewCallback(eventHandler), 1)
	if r1 == 0 {
		fmt.Fprintf(os.Stderr, "setConsoleCtrlHandler failed! Oops\n")
		os.Exit(1)
	}

	fmt.Printf("Setting Handler done. Call result %v %v %v\n", r1, r2, lastErr)

	/*
	 * This is a trivial http server just to keep CF happy
	 */
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hi there, I love you!\n")
	})
	server := &http.Server{
		Addr:              ":8080",
		Handler:           nil,
		ReadHeaderTimeout: 5 * time.Second,
	}
	err := server.ListenAndServe()
	fmt.Fprintf(os.Stderr, "HTTP server exited: %s\n", err)
}
