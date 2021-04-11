package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Service DNS query service
type Service struct {
	Logger     *Logger                      `label:"logger"`
	client     *dns.Client                  `label:"DNS query client"`
	config     *Config                      `label:"config manager"`
	cache      *Cache                       `label:"dns query cache"`
	ptr        []string                     `label:"dns name server ptr"`
	chanExpire chan *dns.Msg                `label:"dns cache need update msg chan"`
	chanItem   chan *CacheItem              `label:"dns query result item chain"`
	mapper     map[string]map[string]net.IP `label:"subdomain mapper to ip list"`
}

// Init dns query service
func (s *Service) Init(test bool) error {
	var err error

	s.chanExpire = make(chan *dns.Msg, 1024)
	s.chanItem = make(chan *CacheItem, 1024)

	// init dns proxy config
	s.config, err = NewConfig(test)
	if nil != err {
		return err
	}

	// init dns client & cache
	s.client = new(dns.Client)
	s.client.Net = "udp"
	s.client.UDPSize = dns.DefaultMsgSize * 2
	s.client.Timeout = time.Millisecond * 600
	s.cache = &Cache{
		MinTTL:   600,
		MaxTTL:   86400,
		MaxCount: 0,
		mu:       new(sync.RWMutex),
		backend:  make(map[string]*CacheItem),
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

// Shutdown dns service
func (s *Service) Shutdown() {
	close(s.chanExpire)
	close(s.chanItem)
}

// Reload config file and reset query cache
func (s *Service) Reload() error {
	return s.Init(false)
}

// Reset query cache
func (s *Service) Reset() {
	s.cache.Reset()
}

// Run dns cache service
func (s *Service) Run() error {
	var err error
	var cKey string
	var idx, num int

	go func() {
		for req := range s.chanExpire {
			cKey = req.Question[0].String() + "|" + strconv.FormatUint(uint64(req.Question[0].Qtype), 10)
			if s.cache.IsExpire(cKey) {
				var group = s.getDomainForwarder(req.Question[0].Name)
				var cnt = len(s.config.Forwarders[group])
				var ctx, _ = context.WithTimeout(context.Background(), s.client.Timeout)

				idx = (idx + 1) % cnt
				var m, err = s.getDnsRecord(ctx, req, s.config.Forwarders[group][idx])
				if nil == err {
					s.chanItem <- m
				} else if nil != err {
					s.Logger.Write(LevelError, " [E] client  query %s error: %s\n", s.toJSON(req.Question), err.Error())
				}
			}
		}
	}()

	go func() {
		for req := range s.chanItem {
			num++
			cKey = req.Msg.Question[0].String() + "|" + strconv.FormatUint(uint64(req.Msg.Question[0].Qtype), 10)
			s.cache.Set(cKey, req)

			if num >= 100 {
				num = 0
				s.cache.GC()
			}
		}
	}()

	return err
}

// Query dns request
func (s *Service) Query(src string, req *dns.Msg) (*dns.Msg, error) {
	defer func() {
		if err := recover(); err != nil {
			s.Logger.Write(LevelEmergency, " [E] client %s trigger panic %v\n", src, err)
		}
	}()

	var resp, err = s.getFromCache(req)
	if err == nil && s.config.Logger.Access {
		s.Logger.Write(LevelRaw, " [T] client %s query cache %s with result %s\n", src, s.toJSON(req.Question), s.toJSON(resp.Answer))
	} else if ErrCacheExpire == err {
		err = nil
		s.chanExpire <- req
	} else if ErrNotFound == err {
		resp, err = s.getFromNet(src, req)
	}

	return resp, err
}

func (s *Service) getFromNet(src string, req *dns.Msg) (*dns.Msg, error) {
	var err error
	var flag bool
	var msg *CacheItem
	var resp *dns.Msg
	var idx = s.config.Rand.Int()
	var respChan = make(chan *CacheItem, s.config.Concurrency)
	var group = s.getDomainForwarder(req.Question[0].Name)
	var cnt = len(s.config.Forwarders[group])
	var ctx, cancel = context.WithTimeout(context.Background(), s.client.Timeout)

	defer func() {
		close(respChan)
		cancel()
	}()

	for i := 0; i < s.config.Concurrency; i++ {
		idx = (idx + i) % cnt

		go func(ctx context.Context, addr string) {
			var m, err = s.getDnsRecord(ctx, req, addr)
			if nil == err && !flag {
				flag = true
				respChan <- m
			} else if nil != err {
				s.Logger.Write(LevelError, " [E] client %s query %s error: %s\n", src, s.toJSON(req.Question), err.Error())
			}
		}(ctx, s.config.Forwarders[group][idx])
	}

	select {
	case msg = <-respChan:
		err = nil
		resp = msg.Msg

		s.chanItem <- msg

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

func (s *Service) getDnsRecord(ctx context.Context, req *dns.Msg, addr string) (*CacheItem, error) {
	var resp, rtt, err = s.client.ExchangeContext(ctx, req, addr)
	if nil == err {
		var ttl = int64(rtt.Seconds())
		if ttl < s.cache.MinTTL {
			ttl = s.cache.MinTTL
		}
		if ttl > s.cache.MaxTTL {
			ttl = s.cache.MaxTTL
		}

		var msg = &CacheItem{
			Msg:    resp,
			Expire: time.Now().Unix() + ttl,
		}

		return msg, nil
	}

	return nil, err
}

// getFromCache query dns from cache
func (s *Service) getFromCache(req *dns.Msg) (*dns.Msg, error) {
	var err error
	var resp *dns.Msg

	// check query ptr
	if nil != s.ptr && dns.TypePTR == req.Question[0].Qtype && dns.ClassINET == req.Question[0].Qclass {
		resp, err = s.getDnsPtr((req))
	}

	// check query host is mapper
	if nil != s.mapper && (dns.TypeA == req.Question[0].Qtype || dns.TypeAAAA == req.Question[0].Qtype) && dns.ClassINET == req.Question[0].Qclass {
		resp, err = s.getDnsMapper((req))
	}

	if nil == resp || ErrNotFound == err {
		var cKey = req.Question[0].String() + "|" + strconv.FormatUint(uint64(req.Question[0].Qtype), 10)
		resp, err = s.cache.Get(cKey)
		if nil != resp {
			resp.Id = req.Id
		}
	}

	return resp, err
}

func (s *Service) getDnsPtr(req *dns.Msg) (*dns.Msg, error) {
	for _, v := range s.ptr {
		if v == req.Question[0].Name {
			var resp = &dns.Msg{
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

	return nil, ErrNotFound
}

func (s *Service) getDnsMapper(req *dns.Msg) (*dns.Msg, error) {
	var resp *dns.Msg
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
				if dns.TypeA == req.Question[0].Qtype {
					resp = &dns.Msg{
						Question: req.Question,
						Answer: []dns.RR{
							&dns.A{
								Hdr: dns.RR_Header{
									Name:     domain + ".",
									Rrtype:   dns.TypeA,
									Class:    req.Question[0].Qclass,
									Ttl:      uint32(s.cache.MinTTL),
									Rdlength: 0x4,
								},
								A: host,
							},
						},
					}
				} else {
					resp = &dns.Msg{
						Question: req.Question,
						Answer:   []dns.RR{},
					}
				}

				resp.Id = req.Id

				return resp, nil
			}

			idx++
			if flag || idx+1 == cnt {
				break
			}
		}
	}

	return nil, ErrNotFound
}

// toJSON convert values to json byte
func (s *Service) toJSON(in interface{}) []byte {
	var ret, _ = json.Marshal(in)

	return ret
}
