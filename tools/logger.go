package tools

import (
	"log"
	"time"
)

var isEnabled = true
var printTimestamp = true

func EnableLogger() {
	isEnabled = true
}

func DisableLogger() {
	isEnabled = false
}

func EnableLoggerTimestamp() {
	printTimestamp = true
}

func DisableLoggerTimestamp() {
	printTimestamp = false
}

func LogOutput(val ...interface{}) {
	if isEnabled {
		if printTimestamp {
			log.Println("[" + time.Now().Format("2006-01-02 15.04:05.000") + "] ")
		}
		log.Println(val...)
	}
}
