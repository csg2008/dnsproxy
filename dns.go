package main

import (
	"github.com/miekg/dns"
)

// NameServer dns name server
type NameServer struct {
	server  *dns.Server
	service *Service
}

// NewNameServer create dns name server
func NewNameServer(service *Service, net string, addr string) (ReverseProxy, bool) {
	var flag bool
	var support = []string{"tcp", "udp"}

	for _, k := range support {
		if k == net {
			flag = true

			break
		}
	}
	if !flag {
		return nil, flag
	}

	var ns = &NameServer{
		server:  new(dns.Server),
		service: service,
	}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", ns.handle)

	ns.server.Addr = addr
	ns.server.Net = net
	ns.server.Handler = mux

	return ns, true
}

// Start server
func (ns *NameServer) Start() error {
	return ns.server.ListenAndServe()
}

// Stop server
func (ns *NameServer) Stop() error {
	return ns.server.Shutdown()
}

// handle query process handle
func (ns *NameServer) handle(w dns.ResponseWriter, req *dns.Msg) {
	defer w.Close()

	if req.MsgHdr.Response {
		return
	}

	var resp, err = ns.service.Query(w.RemoteAddr().String(), req)
	if nil != err {
		ns.service.Logger.Write(LevelError, " [E] client %s query %#v error: %v\n", w.RemoteAddr().String(), req, err)
	} else if nil == resp {
		ns.service.Logger.Write(LevelError, " [E] client %s query %#v result is empty\n", w.RemoteAddr().String(), req)
	} else if err = w.WriteMsg(resp); nil != err {
		ns.service.Logger.Write(LevelError, " [E] send result to client %s %#v error: %v\n", w.RemoteAddr().String(), req, err)
	}
}
