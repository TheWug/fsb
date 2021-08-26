package errorlog

import (
	"log"
	"os"
)

var defaultLogFile  string = "./fsb.log"
var logHandle      *os.File   = nil

func RedirectLog(file string) (error) {
	newFile := defaultLogFile
	if file != "" {
		newFile = file
	}

	newLogHandle, err := os.OpenFile(newFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		log.Printf("error opening file %s for logging: %s",newFile, err.Error())
		return err
	}

	if logHandle != nil {
		logHandle.Close()
	}
	logHandle = newLogHandle

	log.SetOutput(logHandle)
	log.Printf("%s opened for logging.", defaultLogFile)
	return nil
}

func ErrorLog(logger *log.Logger, prefix string, file_and_line string, e error) {
	if e != nil {
		log.Printf("[%-8.s] (%s) Error: %s", prefix, file_and_line, e.Error())
	}
}

func ErrorLogFatal(logger *log.Logger, prefix string, file_and_line string, e error, code int) {
	ErrorLog(logger, prefix, file_and_line, e)

	if e != nil {
		os.Exit(code)
	}
}
