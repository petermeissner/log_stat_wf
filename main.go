package main

import (
	"flag"
	"fmt"
)

func main() {
	// Define command-line flags
	filename := flag.String("file", "test.log", "Path to the log file to process")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	// Additional arguments after flags
	args := flag.Args()

	// print all args
	for _, arg := range args {
		println("arg: " + arg)
	}

	// print flag values
	fmt.Println("Verbose: ", *verbose)
	fmt.Println("filename: ", *filename)

}
