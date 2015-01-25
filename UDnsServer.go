package main

import (
	"flag"
	"github.com/svagner/UDnsServer/config"
	"github.com/svagner/UDnsServer/udns"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
	dnsInst := &udns.DNSServer{Addr: Configuration.Dns.Host, Port: Configuration.Dns.Port}
	fds := strings.Split(Configuration.Dns.ForwardDns, ",")
	if len(fds) > 1 {
		for _, dnshost := range fds {
			fdsData := strings.Split(dnshost, ":")
			port, err := strconv.Atoi(fdsData[1])
			if err != nil {
				log.Println("Failed to add Forward DNS: " + err.Error())
				continue
			}
			dnsInst.AddForwardServer(fdsData[0], fdsData[2], port)
		}
	}
	zonesFiles, err := filepath.Glob(Configuration.Dns.ZonesFiles)
	if err != nil {
		log.Fatalln(err.Error())
	}

	log.Println("Load data from configs:", zonesFiles)
	dnsInst.Start(zonesFiles)

	for {
		select {
		case s := <-sig:
			log.Fatalf("Signal (%s) received, stopping\n", s)
		}
	}
}
