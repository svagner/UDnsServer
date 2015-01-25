package udns

import (
	"bufio"
	"fmt"
	"github.com/tonnerre/golang-dns"
	"log"
	"os"
	"strings"
)

func (self DNSZone) WriteConfig() error {
	result := make([]string, 0)
	if self.TTL != "" {
		result = append(result, "$TTL "+self.TTL)
	}
	for _, soa := range self.Data[dns.TypeSOA] {
		for _, soarec := range soa {
			data := strings.Fields(soarec.String())
			result = append(result, fmt.Sprintf("@\t\t%s\t(", strings.Join(data[2:6], "\t")))
			result = append(result, fmt.Sprintf("\t%d\t;serial", self.Serial+1))
			result = append(result, fmt.Sprintf("\t%s\t;refresh", data[7]))
			result = append(result, fmt.Sprintf("\t%s\t;retry", data[8]))
			result = append(result, fmt.Sprintf("\t%s\t;expire", data[9]))
			result = append(result, fmt.Sprintf("\t%s\t;minimum", data[10]))
			result = append(result, ")")
		}
	}
	result = append(result, "$ORIGIN "+self.Origin)
	result = append(result, ";Nameservers")
	for _, ns := range self.Data[dns.TypeNS] {
		for _, nsrec := range ns {
			result = append(result, strings.Join(strings.Fields(nsrec.String())[2:], "\t"))
		}
	}

	for key, value := range self.Data {
		if key == dns.TypeSOA || key == dns.TypeNS {
			continue
		}
		for _, recs := range value {
			for _, rec := range recs {
				data := strings.Fields(rec.String())
				var nm string
				if key == dns.TypeMX {
					nm = data[0]
				} else {
					nm = strings.TrimRight(data[0], self.Origin)
				}
				result = append(result, nm+"\t"+strings.Join(data[2:], "\t"))
			}
		}
	}
	log.Println(result[0])
	if err := writeLines(result, self.Config+"new"); err != nil {
		return err
	}
	return nil
}

func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}
