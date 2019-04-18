# FSWatch
[![GoDoc](https://godoc.org/github.com/Q1-Energie-AG/fswatch?status.svg)](https://godoc.org/github.com/Q1-Energie-AG/fswatch) [![Go Report Card](https://goreportcard.com/badge/github.com/Q1-Energie-AG/fswatch)](https://goreportcard.com/report/github.com/Q1-Energie-AG/fswatch)


**FSWatch** is like fsnotfiy, but debounces the emitted events for better handling.
E.g. when a (large) file is created, **fswatch** debounces the **Created** event
until the writing is done.

It uses **fsnotfiy** internally.

```console
go get github.com/Q1-Energie-AG/fswatch
```


## Sample code

```go
package main

import (
 	"log"
	"time"
  
	"github.com/q1-energie-ag/fswatch"
	"gopkg.in/fsnotify.v1"
)


func main() {

	// Create a new watcher which waits 10 seconds for new events
	// after the inital event was emitted
	watcher, err := fswatch.NewWatcher(10 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add("/tmp/foo")
	if err != nil {
		log.Fatal(err)
	}
<-done
	
	
}

```
