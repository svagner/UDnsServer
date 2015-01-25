package main

import (
	"flag"
	"github.com/svagner/dnsServer/config"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	var cfgFile = flag.String("config", "config.ini", "Configuration file (ini)")
	flag.Parse()

	Configuration := new(config.Config)

	if err := Configuration.ParseConfig(*cfgFile); err != nil {
		log.Fatalln(err.Error())
	}

	for {
		select {
		case s := <-sig:
			log.Fatalf("Signal (%s) received, stopping\n", s)
		}
	}
}
