package logger

import (
	"fmt"
	"time"
)

func Info(format string, args ...interface{}) {
	log("[INFO] "+format, args...)
}

func Error(format string, args ...interface{}) {
	log("[ERROR] "+format, args...)
}

func log(format string, args ...interface{}) {
	prefix := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf(prefix+" "+format+"\n", args...)
}
