package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"
)

var configFile = flag.String("c", "../conf/proxy.json", "dns proxy server config file")

// DNSFilter dns query filter
type DNSFilter struct {
	Host          string `label:"dns query host"`
	QueryType     uint16 `label:"dns query type"`
	ExactMatching bool   `label:""`
}

// Config dns proxy config option
type Config struct {
	Cache       int                 `json:"cache" label:"dns query cache size"`
	Concurrency int                 `json:"concurrency" label:"spec max concurrency backend forwarder server"`
	Rand        *rand.Rand          `json:"-" label:"forwarder server index"`
	Name        string              `json:"name" label:"dns server name"`
	Pid         string              `json:"pid" label:"pid file path"`
	Logger      *LoggerOption       `json:"logger" label:"logger option"`
	Bind        map[string]string   `json:"bind" label:"dns proxy bind"`
	Rules       map[string]string   `json:"rules" label:"dns query forwarder rule"`
	Forwarders  map[string][]string `json:"forwarders" label:"dns query forwarder server list"`
	Mapper      []string            `json:"mapper" label:"domain to ip mapper"`
	Filters     []DNSFilter         `json:"filters" label:"dns proxy filter rule"`
}

// NewConfig create config object instance
func NewConfig(test bool) (*Config, error) {
	var config = &Config{}

	// try to read the config file
	var bytes, err = ioutil.ReadFile(*configFile)
	if nil == err && nil != bytes {
		err = json.Unmarshal(bytes, &config)
		if err != nil {
			return nil, err
		}

		fmt.Println("proxy: boot from config file", *configFile)
	} else {
		if test {
			return nil, errors.New("proxy: test config file " + *configFile + " failed, config file is not exist or is empty")
		}
	}

	// default bind dns proxy at udp port 53
	if nil == config.Bind || 0 == len(config.Bind) {
		config.Bind = map[string]string{
			"udp":  ":53",
			"http": ":8080",
		}
	}

	config.Rand = rand.New(rand.NewSource(time.Now().Unix()))
	if 0 == config.Cache {
		config.Cache = 256 * 1024 * 1024
	}
	if 0 == config.Concurrency {
		config.Concurrency = 3
	}

	if "" == config.Name {
		config.Name = "dns.proxy.server."
	}
	if !strings.HasSuffix(config.Name, ".") {
		config.Name = config.Name + "."
	}

	// check forwarder rule
	if nil == config.Forwarders {
		config.Forwarders = make(map[string][]string)
	}
	if _, ok := config.Forwarders["normal"]; !ok {
		config.Forwarders["normal"] = []string{"223.5.5.5:53", "223.6.6.6:53", "119.29.29.29:53", "182.254.116.116:53", "101.226.4.6:53", "114.114.114.114:53", "114.114.115.115:53", "202.67.240.222:53", "203.80.96.10:53", "202.45.84.58:53"}
	}
	if _, ok := config.Forwarders["gfw"]; !ok {
		config.Forwarders["gfw"] = []string{"74.82.42.42:53", "107.150.40.234:53", "162.211.64.20:53", "50.116.23.211:53", "50.116.40.226:53", "37.235.1.174:53", "37.235.1.177:53", "8.8.8.8:53", "8.8.4.4:53", "208.67.222.222:53", "208.67.220.220:53", "8.26.56.26:53", "84.200.69.80:53"}
	}
	if nil == config.Rules || 0 == len(config.Rules) {
		config.Rules = map[string]string{
			"default": "normal",
		}
	}
	if _, ok := config.Rules["default"]; !ok {
		return nil, errors.New("proxy: miss default forwarder group rule")
	}
	for k, v := range config.Rules {
		if _, ok := config.Forwarders[v]; !ok {
			if 2 != len(strings.Split(k, ".")) {
				return nil, errors.New("proxy: forwarder rule domain format is xxx.xx, give " + k)
			}

			return nil, errors.New("proxy: domain" + k + "map forwarder" + v + "is not exist")
		}
	}

	// init logger option
	if nil == config.Logger {
		config.Logger = new(LoggerOption)
	}

	return config, nil
}
