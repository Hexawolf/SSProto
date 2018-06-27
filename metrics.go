package main

import (
	"bufio"
	"strings"
	"log"
	"sync"
)

var mut = &sync.Mutex{}

// Check if machine was already logged
func searchForMachine(id string) bool {
	mut.Lock()
	logFile.Seek(0, 0)
	defer logFile.Seek(0, 2)
	defer mut.Unlock()
	scanner := bufio.NewScanner(logFile)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), id) {
			return true
		}
	}
	return false
}

func writeMachine(id string, info []byte) {
	log.Println("HWInfo:", id + ":" + string(info))
}
