package main

import (
	"encoding/json"
	"log"
	"os"
)

func show_version() {
	versionInfo := map[string]string{
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(versionInfo); err != nil {
		log.Fatalf("Error encoding version info: %v", err)
	}
	os.Exit(0)
}
