package dns

import (
	"bufio"
	"errors"
	"github.com/tonnerre/golang-dns"
	//"golang.org/x/exp/fsnotify"
	"github.com/go-fsnotify/fsnotify"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type DNSData map[uint16]DNSRecord
type DNSZones map[string]DNSZone
type DNSRecord map[string][]dns.RR
type ForwardServers []ForwardResolver

type DNSZone struct {
	Origin string
	Config string
	Data   DNSData
	Serial uint64
}

type ForwardResolver struct {
	server string
	proto  string
}

type DNSServer struct {
	Addr      string
	Port      int
	Forwarder ForwardServers
}

func (self DNSZone) Handler(w dns.ResponseWriter, req *dns.Msg) {
	question := req.Question[0]
	if _, ok := self.Data[question.Qtype][question.Name]; ok {
		req.Answer = self.Data[question.Qtype][question.Name]
	}
	req.Response = true
	w.WriteMsg(req)
}

func (self *DNSZone) GetOrigin(zonefile string) error {
	f, err := os.Open(zonefile)
	defer f.Close()
	if err != nil {
		return err
	}
	readln := bufio.NewScanner(f)
	for readln.Scan() {
		if strings.Index(readln.Text(), "$ORIGIN") == 0 {
			org := strings.Split(readln.Text(), " ")
			if len(org) > 1 {
				self.Origin = org[1]
				self.Config = zonefile
				return nil
			}
		}
	}
	return errors.New("Origin record wasn't found")
}

func (self *DNSZone) ReloadDNSZone(zoneFile string) {
	f, err := os.Open(zoneFile)
	defer f.Close()
	if err != nil {
		log.Println(err)
		return
	}
	newData := make(DNSData)
	var newSerial uint64
	for b := range dns.ParseZone(f, "", zoneFile) {
		if b.Error != nil {
			log.Println("Error")
			continue
		}
		if _, ok := newData[b.RR.Header().Rrtype]; !ok {
			newData[b.RR.Header().Rrtype] = make(DNSRecord, 0)
		}
		if b.RR.Header().Name == "." {
			(*b.RR.Header()).Name = self.Origin

		}
		if _, ok := newData[b.RR.Header().Rrtype][b.RR.Header().Name]; !ok {
			newData[b.RR.Header().Rrtype][b.RR.Header().Name] = make([]dns.RR, 0)
		}
		newData[b.RR.Header().Rrtype][b.RR.Header().Name] = append(newData[b.RR.Header().Rrtype][b.RR.Header().Name], b.RR)
		if b.RR.Header().Rrtype == dns.TypeSOA {
			newSerial, err = strconv.ParseUint(strings.Fields(b.String())[6], 0, 64)
			if err != nil {
				log.Println(err.Error())
			}
		}
	}
	if self.Serial < newSerial {
		self.Data = newData
		log.Println(zoneFile, "was reloaded:",
			len(self.Data[dns.TypeA]), "A records",
			len(self.Data[dns.TypeSOA]), "SOA records",
			len(self.Data[dns.TypeNS]), "NS records",
			len(self.Data[dns.TypePTR]), "PTR records")
	}
}

func (self *DNSZone) ParseDNSZone(zoneFile string) {
	f, err := os.Open(zoneFile)
	defer f.Close()
	if err != nil {
		log.Println(err)
		return
	}
	self.Data = make(DNSData)
	for b := range dns.ParseZone(f, "", zoneFile) {
		if b.Error != nil {
			log.Println("Error")
			continue
		}
		if _, ok := self.Data[b.RR.Header().Rrtype]; !ok {
			self.Data[b.RR.Header().Rrtype] = make(DNSRecord, 0)
		}
		if b.RR.Header().Name == "." {
			(*b.RR.Header()).Name = self.Origin

		}
		if _, ok := self.Data[b.RR.Header().Rrtype][b.RR.Header().Name]; !ok {
			self.Data[b.RR.Header().Rrtype][b.RR.Header().Name] = make([]dns.RR, 0)
		}
		self.Data[b.RR.Header().Rrtype][b.RR.Header().Name] = append(self.Data[b.RR.Header().Rrtype][b.RR.Header().Name], b.RR)
		if b.RR.Header().Rrtype == dns.TypeSOA {
			self.Serial, err = strconv.ParseUint(strings.Fields(b.String())[6], 0, 64)
			if err != nil {
				log.Println(err.Error())
			}
		}
	}
	log.Println(zoneFile, "was loaded:",
		len(self.Data[dns.TypeA]), "A records",
		len(self.Data[dns.TypeSOA]), "SOA records",
		len(self.Data[dns.TypeNS]), "NS records",
		len(self.Data[dns.TypePTR]), "PTR records")
}

func (self ForwardServers) Lookup(w dns.ResponseWriter, req *dns.Msg) {
	for _, server := range self {
		client := &dns.Client{
			Net:          server.proto,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		in, _, err := client.Exchange(req, server.server)
		if err == nil {
			w.WriteMsg(in)

			return
		}
		log.Println(err)
	}
}

func (self *DNSServer) AddForwardServer(host string, proto string, port int) error {
	if ip := net.ParseIP(host); ip == nil {
		return errors.New("IP address " + host + " isn't valid")
	}
	if proto != "udp" && proto != "tcp" {
		return errors.New("Proto for forward server " + net.JoinHostPort(host, strconv.Itoa(port)) + " isn't valid")
	}
	self.Forwarder = append(self.Forwarder, ForwardResolver{proto: proto, server: net.JoinHostPort(host, strconv.Itoa(port))})
	return nil
}

func (self DNSServer) Start(zoneConfigs []string) {
	udpHandler := dns.NewServeMux()
	tcpHandler := dns.NewServeMux()
	if len(self.Forwarder) > 0 {
		tcpHandler.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) { self.Forwarder.Lookup(w, req) })
	}
	for _, zoneName := range zoneConfigs {
		zone := new(DNSZone)
		if err := zone.GetOrigin(zoneName); err != nil {
			log.Println(err)
			continue
		}
		go zone.ConfigMonitor()
		zone.ParseDNSZone(zoneName)

		udpHandler.HandleFunc(zone.Origin, func(w dns.ResponseWriter, req *dns.Msg) { zone.Handler(w, req) })
		tcpHandler.HandleFunc(zone.Origin, func(w dns.ResponseWriter, req *dns.Msg) { zone.TransferHandler(w, req) })
	}

	udpServer := &dns.Server{
		Addr:         net.JoinHostPort(self.Addr, strconv.Itoa(self.Port)),
		Net:          "udp",
		Handler:      udpHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	tcpServer := &dns.Server{
		Addr:         net.JoinHostPort(self.Addr, strconv.Itoa(self.Port)),
		Net:          "tcp",
		Handler:      tcpHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
}

func (self *DNSZone) ConfigMonitor() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err.Error())
	}
	defer watcher.Close()
	done := make(chan bool)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					if event.Name == self.Config {
						self.ReloadDNSZone(self.Config)
						log.Println("Zone was reloaded from file", self.Config, "[", event, "]")
					}
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
				done <- true
			}
		}
	}()

	err = watcher.Add(path.Dir(self.Config))
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

func (self DNSZone) TransferHandler(w dns.ResponseWriter, req *dns.Msg) {

	value := req.Question[0].Qtype
	switch value {
	case dns.TypeAXFR, dns.TypeIXFR:
		c := make(chan *dns.Envelope)
		defer close(c)
		var e *error
		err := dns.TransferOut(w, req, c, e)
		if err != nil {
			log.Printf("Could not begin zone transfer.")
			return
		}
		w.Hijack()
		records := make([]dns.RR, 0)
		for _, types := range self.Data {
			for _, value := range types {
				for _, rec := range value {
					records = append(records, rec)
				}
			}
		}
		c <- &dns.Envelope{RR: records}
		c <- &dns.Envelope{RR: self.Data[dns.TypeSOA][req.Question[0].Name]}
		return
	default:
		log.Printf("Was not a zone transfer request.")
	}

}
