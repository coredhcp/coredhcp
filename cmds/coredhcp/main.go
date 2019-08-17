package main

import (
	"time"

	"github.com/coredhcp/coredhcp"
	"github.com/coredhcp/coredhcp/config"
	"github.com/coredhcp/coredhcp/logger"
	_ "github.com/coredhcp/coredhcp/plugins/DNS"
	_ "github.com/coredhcp/coredhcp/plugins/IPv4"
	_ "github.com/coredhcp/coredhcp/plugins/example"
	_ "github.com/coredhcp/coredhcp/plugins/file"
	_ "github.com/coredhcp/coredhcp/plugins/server_id"
)

func main() {
	logger := logger.GetLogger()
	config, err := config.Load()
	if err != nil {
		logger.Fatal(err)
	}
	server := coredhcp.NewServer(config)
	if err := server.Start(); err != nil {
		logger.Fatal(err)
	}
	if err := server.Wait(); err != nil {
		logger.Print(err)
	}
	time.Sleep(time.Second)
}
