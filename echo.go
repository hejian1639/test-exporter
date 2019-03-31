package main

import (
	log "github.com/sirupsen/logrus"
	"os"
)

func main () {
	file, err := os.OpenFile("./logrus.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err == nil {
		log.SetOutput(file)
	} else {
		log.Info("Failed to log to file, using default stderr")
	}


	log.Info("A walrus appears")
}