package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/BurntSushi/toml"
)

var reportFilePath = flag.String("path", "report/report.toml", "path to report.toml")

func main() {
	flag.Parse()

	report := struct {
		Image struct {
			Digest string `toml:"digest,omitempty"`
		} `toml:"image"`
	}{}
	_, err := toml.DecodeFile(*reportFilePath, &report)
	if err != nil {
		log.Fatal(err, "error decoding report toml file")
	}

	fmt.Println(report.Image.Digest)
}
