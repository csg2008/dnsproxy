package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/miekg/dns"
)

// HTTPServer http dns server
type HTTPServer struct {
	net     string
	service *Service
	server  *http.Server
}

// NewHTTPServer http base dns server
func NewHTTPServer(service *Service, net string, addr string) (ReverseProxy, bool) {
	var flag bool
	var support = []string{"http", "https"}

	for _, k := range support {
		if k == net {
			flag = true

			break
		}
	}
	if !flag {
		return nil, flag
	}

	var ns = &HTTPServer{
		net:     net,
		service: service,
		server: &http.Server{
			Addr:           addr,
			Handler:        http.DefaultServeMux,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}

	return ns, true
}

// Start server
func (s *HTTPServer) Start() error {
	var mux = http.NewServeMux()
	mux.HandleFunc("/", s.resolveDNS)

	s.server.Handler = mux

	return s.server.ListenAndServe()
}

// Stop server
func (s *HTTPServer) Stop() error {
	return s.server.Shutdown(nil)
}

// resolveDNS process dns query
func (s *HTTPServer) resolveDNS(w http.ResponseWriter, req *http.Request) {
	if "GET" == req.Method {
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := req.ParseForm(); nil != err {
			w.Write([]byte("{\"code\":1001, \"message\":\"parse query failed, " + err.Error() + "\"}"))
			s.service.Logger.Write(LevelError, " [E] client %s parse query failed: %v\n", req.RemoteAddr, err)
			return
		}

		var queryName = req.FormValue("name")
		var queryType, err = strconv.Atoi(req.FormValue("type"))
		if err != nil {
			queryType = 255
		}

		var resp *dns.Msg
		var msg = new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(queryName), uint16(queryType))
		msg.RecursionDesired = true

		if resp, err = s.service.Query(req.RemoteAddr, msg); nil != err {
			w.Write([]byte("{\"code\":1002, \"message\":\"query failed, " + err.Error() + "\"}"))
			s.service.Logger.Write(LevelError, " [E] client %s query %#v error: %v\n", req.RemoteAddr, msg, err)
			return
		}

		var retJSON = map[string]interface{}{
			"Answer": resp.Answer,
			"Name":   queryName,
			"Type":   dns.TypeToString[uint16(queryType)],
			"Code":   resp.Rcode,
		}

		if retByte, err := json.Marshal(retJSON); err != nil {
			w.Write([]byte("{\"code\":1003, \"message\":\"serialize query result failed, " + err.Error() + "\"}"))
			s.service.Logger.Write(LevelError, " [E] client %s serialize query result failed: %v\n", req.RemoteAddr, msg, err)
		} else {
			w.Write(retByte)
		}
	}
}
