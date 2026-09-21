package main

import (
	"fmt"
	"os"
)

var version = "v1.19.27"
var fail = "false"

func main() {
	if len(os.Args) != 2 || os.Args[1] != "-v" {
		os.Exit(2)
	}
	fmt.Println("Mihomo", version)
	if fail == "true" {
		os.Exit(1)
	}
}
