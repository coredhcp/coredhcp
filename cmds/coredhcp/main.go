package main

import (
	"log"
	"time"

	"github.com/coredhcp/coredhcp"
	"github.com/coredhcp/coredhcp/config"
	_ "github.com/coredhcp/coredhcp/plugins/file"
	_ "github.com/coredhcp/coredhcp/plugins/server_id"
)

// Application variables
var (
	AppName    = "CoreDHCP"
	AppVersion = "v0.1"
)

func main() {
	config, err := config.Parse()
	if err != nil {
		log.Fatal(err)
	}
	server := coredhcp.NewServer(config)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
	if err := server.Wait(); err != nil {
		log.Print(err)
	}
	time.Sleep(time.Second)
}
