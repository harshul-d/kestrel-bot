package main

import (
	"os"

	"aurorapilot/pilot"
)

func main() {
	os.Exit(pilot.Run(os.Stdin, os.Stdout))
}
