package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Service DNS query service
type Service struct {
	Logger *Logger                      `label:"logger"`
	client *dns.Client                  `label:"DNS query client"`
	config *Config                      `label:"config manager"`
	cache  *Cache                       `label:"dns query cache"`
	ptr    []string                     `label:"dns name server ptr"`
	mapper map[string]map[string]net.IP `label:"subdomain mapper to ip list"`
}

// Init dns query service
func (s *Service) Init(test bool) error {
	var err error

	// init dns proxy config
	s.config, err = NewConfig(test)
	if nil != err {
		return err
	}

	// init dns client & cache
	s.client = new(dns.Client)
	s.client.Net = "udp"
	s.client.Timeout = time.Millisecond * 400
	s.cache = &Cache{
		MinTTL:   600,
		MaxTTL:   86400,
		MaxCount: 0,
		mu:       new(sync.RWMutex),
		backend:  make(map[string]*DNSMsg),
	}

	// init query log
	s.Logger = &Logger{config: s.config}
	if err = s.Logger.Init(); nil != err {
		return err
	}

	// init dns ptr
	var addrs []net.Addr
	addrs, err = net.InterfaceAddrs()
	if nil != err {
		return err
	}
	if len(addrs) > 0 {
		s.ptr = make([]string, len(addrs))
	}
	for _, addr := range addrs {
		var dot string
		var sub []string
		var tmp = strings.Split(addr.String(), "/")

		if -1 == strings.Index(tmp[0], "::") {
			if -1 == strings.Index(tmp[0], ".") {
				dot = ":"
				sub = strings.Split(tmp[0], ":")
			} else {
				dot = "."
				sub = strings.Split(tmp[0], ".")
			}

			var val string
			var cnt = len(sub)
			var idx = cnt
			for {
				idx--
				if idx < 0 {
					break
				}

				val = val + sub[idx] + dot
			}

			s.ptr = append(s.ptr, val+"in-addr.arpa.")
		}
	}

	// init subdomain mapper
	if nil != s.config.Mapper && len(s.config.Mapper) > 0 {
		s.mapper = make(map[string]map[string]net.IP, len(s.config.Mapper))
		for _, rule := range s.config.Mapper {
			var val = strings.SplitN(rule, ":", 2)
			if 2 != len(val) {
				return errors.New("proxy: mapper rule format is domain:value, give " + rule)
			}

			var sub = strings.Split(val[0], ".")
			var subLen = len(sub)
			if subLen < 2 {
				return errors.New("proxy: mapper domain miss root part, give " + val[0])
			}

			var key = sub[subLen-2] + "." + sub[subLen-1]
			if _, ok := s.mapper[key]; !ok {
				s.mapper[key] = make(map[string]net.IP)
			}
			s.mapper[key][val[0]] = net.ParseIP(val[1])
		}
	}

	return nil
}

// Reload config file and reset query cache
func (s *Service) Reload() error {
	return s.Init(false)
}

// Reset query cache
func (s *Service) Reset() {
	s.cache.Reset()
}

// Query dns request
func (s *Service) Query(src string, req *dns.Msg) (*dns.Msg, error) {
	// 错误捕获
	defer func() {
		if err := recover(); err != nil {
			s.Logger.Write(LevelEmergency, " [E] client %s trigger panic %v\n", src, err)
		}
	}()

	var resp, err = s.getFromCache(req)
	if err == nil {
		if s.config.Logger.Access {
			s.Logger.Write(LevelRaw, " [T] client %s query cache %s with result %s\n", src, s.toJSON(req.Question), s.toJSON(resp.Answer))
		}

		return resp, err
	}

	var flag bool
	var msg *DNSMsg
	var idx = s.config.Rand.Int()
	var respChan = make(chan *DNSMsg, s.config.Concurrency)
	var group = s.getDomainForwarder(req.Question[0].Name)
	var cnt = len(s.config.Forwarders[group])
	var ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*500)

	defer func() {
		close(respChan)
		cancel()
	}()

	for i := 0; i < s.config.Concurrency; i++ {
		idx = (idx + i) % cnt

		go func(ctx context.Context, addr string) {
			var resp, rtt, err = s.client.ExchangeContext(ctx, req, addr)
			if nil == err && !flag {
				flag = true

				var ttl = int64(rtt.Seconds())
				if ttl < s.cache.MinTTL {
					ttl = s.cache.MinTTL
				}
				if ttl > s.cache.MaxTTL {
					ttl = s.cache.MaxTTL
				}

				var msg = &DNSMsg{
					Msg:    resp,
					Expire: time.Now().Unix() + ttl,
				}

				respChan <- msg
			} else if nil != err {
				s.Logger.Write(LevelError, " [E] client %s query %s error: %s\n", src, s.toJSON(req.Question), err.Error())
			}
		}(ctx, s.config.Forwarders[group][idx])
	}

	select {
	case msg = <-respChan:
		err = nil
		resp = msg.Msg
		if len(req.Question) > 0 {
			s.cache.Set(req.Question[0].String(), msg)
		}

		if s.config.Logger.Access {
			s.Logger.Write(LevelRaw, " [T] client %s query remote %s with result %s\n", src, s.toJSON(req.Question), s.toJSON(resp.Answer))
		}
	case <-ctx.Done():
		err = ErrCacheTimeout
	}

	return resp, err
}

// getDomainForwarder get domain mapper forwarder group name
func (s *Service) getDomainForwarder(domain string) string {
	var ret string
	var host = strings.Trim(strings.TrimRight(strings.ToLower(domain), "dhcp\\ host."), ".")
	var sub = strings.Split(host, ".")
	var cnt = len(sub)

	if cnt >= 2 {
		var key = sub[cnt-2] + "." + sub[cnt-1]
		ret = s.config.Rules[key]
	}
	if "" == ret {
		ret = s.config.Rules["default"]
	}

	return ret
}

// getFromCache query dns from cache
func (s *Service) getFromCache(req *dns.Msg) (*dns.Msg, error) {
	var resp *dns.Msg

	// check query ptr
	if nil != s.ptr && dns.TypePTR == req.Question[0].Qtype && dns.ClassINET == req.Question[0].Qclass {
		for _, v := range s.ptr {
			if v == req.Question[0].Name {
				resp = &dns.Msg{
					Question: req.Question,
					Answer: []dns.RR{
						&dns.PTR{
							Hdr: dns.RR_Header{
								Name:     req.Question[0].Name,
								Rrtype:   req.Question[0].Qtype,
								Class:    req.Question[0].Qclass,
								Ttl:      uint32(s.cache.MinTTL),
								Rdlength: uint16(strings.Count(s.config.Name, "")),
							},
							Ptr: s.config.Name,
						},
					},
				}

				resp.Rcode = dns.RcodeSuccess
				resp.Id = req.Id
				return resp, nil
			}
		}
	}

	// check query host is mapper
	if nil != s.mapper && dns.TypeA == req.Question[0].Qtype && dns.ClassINET == req.Question[0].Qclass {
		var domain = strings.Trim(strings.TrimRight(strings.ToLower(req.Question[0].Name), "dhcp\\ host."), ".")
		var sub = strings.Split(domain, ".")
		var cnt = len(sub)
		var key = sub[cnt-2] + "." + sub[cnt-1]

		if items, ok := s.mapper[key]; ok {
			var idx int
			var flag bool
			var host net.IP
			var tmp string

			for {
				tmp = strings.Join(sub[idx:cnt], ".")

				if host, ok = items[tmp]; ok {
					flag = true
				} else if host, ok = items["."+tmp]; ok {
					flag = true
				}

				if flag {
					resp = &dns.Msg{
						Question: req.Question,
						Answer: []dns.RR{
							&dns.A{
								Hdr: dns.RR_Header{
									Name:     domain + ".",
									Rrtype:   0x1,
									Class:    0x1,
									Ttl:      uint32(s.cache.MinTTL),
									Rdlength: 0x4,
								},
								A: host,
							},
						},
					}

					resp.Id = req.Id
					return resp, nil
				}

				idx++
				if idx+1 == cnt {
					break
				}
			}
		}
	}

	var msg, ok = s.cache.Get(req.Question[0].String())
	if !ok || nil == msg {
		return nil, ErrNotFound
	}

	resp = new(dns.Msg)
	*resp = *msg
	resp.Id = req.Id

	return resp, nil
}

// toJSON convert values to json byte
func (s *Service) toJSON(in interface{}) []byte {
	var ret, _ = json.Marshal(in)

	return ret
}
