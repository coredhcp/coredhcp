// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"flag"
	"time"

	"github.com/coredhcp/coredhcp"
	"github.com/coredhcp/coredhcp/config"
	"github.com/coredhcp/coredhcp/logger"
	_ "github.com/coredhcp/coredhcp/plugins/dns"
	_ "github.com/coredhcp/coredhcp/plugins/file"
	_ "github.com/coredhcp/coredhcp/plugins/lease_time"
	_ "github.com/coredhcp/coredhcp/plugins/netmask"
	_ "github.com/coredhcp/coredhcp/plugins/range"
	_ "github.com/coredhcp/coredhcp/plugins/router"
	_ "github.com/coredhcp/coredhcp/plugins/server_id"
	_ "github.com/coredhcp/plugins/redis"
	"github.com/sirupsen/logrus"
)

var (
	flagLogFile     = flag.String("logfile", "", "Name of the log file to append to. Default: stdout/stderr only")
	flagLogNoStdout = flag.Bool("nostdout", false, "Disable logging to stdout/stderr")
	flagDebug       = flag.Bool("debug", false, "Enable debug output")
)

func main() {
	flag.Parse()
	log := logger.GetLogger("main")
	if *flagDebug {
		log.Logger.SetLevel(logrus.DebugLevel)
		log.Infof("Enabled debug logging")
	}
	if *flagLogFile != "" {
		log.Infof("Logging to file %s", *flagLogFile)
		logger.WithFile(log, *flagLogFile)
	}
	if *flagLogNoStdout {
		log.Infof("Disabling logging to stdout/stderr")
		logger.WithNoStdOutErr(log)
	}
	config, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	server := coredhcp.NewServer(config)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
	if err := server.Wait(); err != nil {
		log.Error(err)
	}
	time.Sleep(time.Second)
}
