package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	mode     = flag.String("mode", "wait", "One of: wait or copy")
	to       = flag.String("to", "", "Where to copy this binary")
	waitFile = flag.String("wait-file", "", "file to wait on")
	doneFile = flag.String("done-file", "", "file to write on completion")
	execute  = flag.String("execute", "", "What to run after waiting")
	timeout  = flag.String("timeout", "60m", "How long to wait")
	errFile  = flag.String("error-file", "", "Location of shared error file")
)

func main() {
	flag.Parse()

	switch *mode {
	case "wait":
		if *doneFile == "" {
			log.Fatal("need -done-file with -mode=wait")
		}
		if *execute == "" {
			log.Fatal("need -execute with -mode=wait")
		}

		exists, err := exists(*waitFile)
		if err != nil {
			log.Fatal(err)
		}

		if exists {
			log.Fatal("file specified for writing already exists")
		}

		if *waitFile != "" {
			log.Printf("waiting on %s\n", *waitFile)
			err = waitForStep(*waitFile)
			if err != nil {
				log.Fatal(err)
			}
		}

		split := strings.Split(*execute, " ")

		cmd := exec.Command(split[0], split[1:]...)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			if *errFile != "" {
				os.Create(*errFile)
			}
			log.Fatalf("error running command %s", err.Error())
		}

		err = ioutil.WriteFile(*doneFile, []byte("done"), 0666)
		if err != nil {
			log.Fatalf("error writing file %s", err.Error())
		}
	case "copy":
		if *to == "" {
			log.Fatal("-to must be specified with -mode=copy")
		}
		binaryPath, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		from, err := os.Open(binaryPath)
		if err != nil {
			log.Fatal(err)
		}
		defer from.Close()

		needsDelete, err := exists(*to)
		if err != nil {
			log.Fatal(err)
		}

		if needsDelete {
			err := os.RemoveAll(*to)
			if err != nil {
				log.Fatal(err)
			}
		}

		to, err := os.OpenFile(*to, os.O_RDWR|os.O_CREATE, 0777)
		if err != nil {
			log.Fatal(err)
		}
		defer to.Close()

		_, err = io.Copy(to, from)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func exists(fileName string) (bool, error) {
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err != nil, err
}

func waitForStep(filepath string) error {
	duration, err := time.ParseDuration(*timeout)
	if err != nil {
		return err
	}
	timeoutChan := time.After(duration)
	tickerChan := time.NewTicker(time.Second)
	defer tickerChan.Stop()

	for {
		select {
		case <-timeoutChan:
			return errors.New("timeout waiting for previous step to complete")
		case <-tickerChan.C:
			if *errFile != "" {
				if _, err := os.Stat(*errFile); err == nil {
					log.Fatalf("another step has errored")
				}
			}
			if fi, err := os.Stat(filepath); err == nil {
				//downward api places a file even if the annotation is empty, so we need to wait for non-empty file
				if fi.Size() > 0 {
					return nil
				}
			}
		}
	}
}
