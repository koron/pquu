package main

import (
	"log"
	"os"

	"github.com/koron-go/subcmd"
)

var rootSet = subcmd.DefineRootSet(
	pushCommand,
)

func main() {
	err := subcmd.Run(rootSet, os.Args[1:]...)
	if err != nil {
		log.Fatal(err)
	}
}
