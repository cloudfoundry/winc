The goshut.exe has a `CTRL_SHUTDOWN_EVENT` handler and will keep printing forever when it receives this event. To be used for testing purposes.

Src: https://github.com/greenhouse-org/graceful-shutdown-sample-apps

The original is readme is as follows:

---

# goshut

This is a sample binary app written in Go to demonstrate graceful shutdown in windows containers.

## Build
You can use the `goshut.exe` binary in the repo, or you can build it yourself by `cd`ing into the root of the app on a Windows machine, and running:
`go build -o .\goshut.exe .\goshut.go`

## Usage
1. `cf push` your app using:
`cf push goshut -s windows -b binary_buildpack -c "./goshut.exe"`

1. Run:
`cf logs goshut`

1. In a seperate terminal run:
`cf stop goshut`

1. If you look in the terminal where you are watching the logs you should see output being printed every half a second for ~5 seconds.

Sample output:
```
[CELL/0] OUT Cell 06da2a35-2bd5-4c06-ac24-a692e9940156 destroying container for instance 0a669469-eaa5-4ce9-6d52-27dc
[APP/PROC/WEB/0] OUT consoleControlHandler called with 6
[APP/PROC/WEB/0] OUT Received CTRL_SHUTDOWN_EVENT (6)
[APP/PROC/WEB/0] OUT Looping forever
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=0s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=515.0973ms
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=1.0267962s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=1.53111s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=2.0423025s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=2.5461696s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=3.0607809s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=3.5695714s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=4.0758866s
[APP/PROC/WEB/0] OUT IN LOOP: Elapsed time=4.578698s
[CELL/0] OUT Cell 06da2a35-2bd5-4c06-ac24-a692e9940156 successfully destroyed container for instance 0a669469-eaa5-4ce9-6d52-27dc
```
