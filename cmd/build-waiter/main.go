package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	mode     = flag.String("mode", "wait", "one of: wait or copy")
	to       = flag.String("to", "", "where to copy this binary")
	waitFile = flag.String("wait-file", "", "file to wait on")
	doneFile = flag.String("done-file", "", "file to write on completion")
	execute  = flag.String("execute", "", "What to run after waiting")
	errFile  = flag.String("error-file", "", "shared error file")
)

func main() {
	flag.Parse()

	switch *mode {
	case "wait":
		if *errFile != "" {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go watchForErrors(ctx, *errFile)
		}

		err := wait(*waitFile, *doneFile, *execute)
		if err != nil {
			exitWithError(err.Error(), *errFile)
		}

	case "copy":
		err := copy(*to)
		if err != nil {
			exitWithError(err.Error(), *errFile)
		}
	}
}

func wait(waitFile, doneFile, execute string) error {
	if execute == "" {
		return errors.New("need -execute with -mode=wait")
	}

	fileExists, err := exists(doneFile)
	if err != nil {
		return err
	}

	if fileExists {
		return errors.New("file specified for writing already exists")
	}

	if waitFile != "" {
		log.Printf("waiting on %s\n", waitFile)
		waitForStep(waitFile)
	}

	split := strings.Split(execute, " ")

	cmd := exec.Command(split[0], split[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running command %s", err.Error())
	}

	if doneFile != "" {
		err = os.WriteFile(doneFile, []byte("done"), 0666)
		if err != nil {
			return fmt.Errorf("error writing file %s", err.Error())
		}
	}
	return nil
}

func copy(to string) error {
	if to == "" {
		log.Fatal("-to must be specified with -mode=copy")
	}
	binaryPath, err := os.Executable()
	if err != nil {
		return err
	}
	srcFile, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	needsDelete, err := exists(to)
	if err != nil {
		return err
	}

	if needsDelete {
		if err := os.RemoveAll(to); err != nil {
			return err
		}
	}

	destFile, err := os.OpenFile(to, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

func waitForStep(waitPath string) {
	tickerChan := time.NewTicker(time.Second)
	defer tickerChan.Stop()

	for range tickerChan.C {
		if fi, err := os.Stat(waitPath); err == nil && fi.Size() > 0 {
			return
		}
	}
}

func exists(fileName string) (bool, error) {
	if _, err := os.Stat(fileName); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func exitWithError(message, errorFile string) {
	if errorFile != "" {
		os.WriteFile(errorFile, []byte(message), 0666)
	}

	log.Fatal(message)
}

func watchForErrors(ctx context.Context, errFile string) {
	tickerChan := time.NewTicker(time.Second)
	defer tickerChan.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerChan.C:
			if _, err := os.Stat(errFile); err == nil {
				log.Fatal("another step has errored")
			}
		}
	}
}
