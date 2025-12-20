package main

import (
	"log"
	"os"
)

var (
	debugLog *log.Logger
)

func initLogger(debug bool) {
	if debug {
		debugLog = log.New(os.Stderr, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		debugLog = log.New(os.NewFile(0, os.DevNull), "", 0)
	}
}
