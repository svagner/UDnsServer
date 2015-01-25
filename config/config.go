package config

import "code.google.com/p/gcfg"

type Config struct {
	Dns struct {
		Host       string
		Port       int
		ZonesFiles string
	}
	Global struct {
		LogFile string
	}
	Scanner ScanConfig
}

type ScanConfig struct {
	Network          string
	Mask             int
	ZabbixClientPort int
	AnswerTimeout    float64
	HostConcurency   int
}

func (self *Config) ParseConfig(file string) error {
	if err := gcfg.ReadFileInto(self, file); err != nil {
		return err
	}
	return nil
}
