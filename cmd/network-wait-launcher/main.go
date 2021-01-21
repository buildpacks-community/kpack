package main

import (
	"errors"
	"log"
	"net"
	"os"
	"os/exec"
	"time"
)

func main() {
	// example: /kpack/network-wait-launcher gcr.io -- completion --notary-url="some-url"

	hostname := os.Args[1]
	command := os.Args[3:]

	if err := waitForDns(hostname); err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(command[0], command[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

}

func waitForDns(hostname string) error {
	timeoutChan := time.After(10 * time.Second)
	tickerChan := time.NewTicker(time.Second)
	defer tickerChan.Stop()

	for {
		select {
		case <-timeoutChan:
			return errors.New("timeout waiting for network")
		case <-tickerChan.C:
			_, err := net.LookupIP(hostname)
			if err == nil {
				return nil
			}
		}
	}

}
