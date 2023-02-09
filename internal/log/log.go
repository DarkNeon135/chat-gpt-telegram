package log

import (
	"fmt"
	"io"
	"log"
	"os"
)

var (
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

const (
	logsDir     = "logs"
	loggerFlags = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile | log.Lmsgprefix
)

func Init(logFileName string, writeToConsole bool) {
	if len(logFileName) > 64 {
		panic("Too long log file name")
	}

	var logStream io.Writer
	if len(logFileName) == 0 && !writeToConsole {
		panic("Logger initialization failed")
	} else {
		if len(logFileName) != 0 {
			createLogsDir()
			fStream, err := os.OpenFile(fmt.Sprintf("%s/%s.log", logsDir, logFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0777)
			if err != nil {
				panic(err)
			}
			logStream = io.MultiWriter(fStream)
		}

		if writeToConsole {
			if len(logFileName) != 0 {
				logStream = io.MultiWriter(logStream, os.Stdout)
			} else {
				logStream = os.Stdout
			}
		}
	}

	Info = log.New(logStream, "[INFO]: ", loggerFlags)
	Warning = log.New(logStream, "[WARNING]: ", loggerFlags)
	Error = log.New(logStream, "[ERROR]: ", loggerFlags)
}

func createLogsDir() {
	if _, err := os.Stat(logsDir); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(logsDir, 0777)
			if err != nil {
				panic(err.Error())
			}
		} else {
			panic(err.Error())
		}
	}
}
