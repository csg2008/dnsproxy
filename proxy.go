package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var testModel = flag.Bool("t", false, "test dns proxy server config file")

// ProxyHandle dns query handle
type ProxyHandle func(s *Service, net string, addr string) (ReverseProxy, bool)

// ReverseProxy DNS reverse proxy server
type ReverseProxy interface {
	Start() error
	Stop() error
}

// Proxy dns proxy
type Proxy struct {
	status      int
	exitConfirm int
	interrupt   chan os.Signal
	wg          *sync.WaitGroup
	service     *Service
	version     map[string]string
	provider    map[string]ReverseProxy
}

// NewProxy create dns proxy server
func NewProxy(ver map[string]string) *Proxy {
	return &Proxy{
		wg:      new(sync.WaitGroup),
		service: new(Service),
		version: ver,
	}
}

// init proxy server
func (p *Proxy) init(test bool) error {
	fmt.Println("proxy: init dns proxy service")
	fmt.Println("proxy: current ", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("proxy: version:", p.version["version"], "build:", p.version["date"], "rev:", p.version["rev"])

	var err = p.service.Init(test)
	if nil != err {
		return err
	}

	p.interrupt = make(chan os.Signal, 1)
	p.provider = make(map[string]ReverseProxy, len(p.service.config.Bind))
	for net, addr := range p.service.config.Bind {
		for _, handle := range provider {
			if obj, ok := handle(p.service, net, addr); ok {
				p.provider[net] = obj

				break
			}
		}
	}

	return nil
}

// Run proxy server
func (p *Proxy) Run() error {
	var err = p.init(*testModel)
	if nil != err {
		return err
	}

	if *testModel {
		return errors.New("proxy: config file test ok")
	}

	if 0 == p.status {
		p.status = 1

		if "" != p.service.config.Pid {
			err = p.createPidFile(p.service.config.Pid)
			if nil != err {
				return err
			}
		}

		if err = p.service.Run(); nil != err {
			return err
		}

		for k, v := range p.provider {
			go func(net string, handle ReverseProxy) {
				fmt.Println("proxy: starting ", net, " at socket ", p.service.config.Bind[net])

				p.wg.Add(1)
				var err = handle.Start()
				if nil != err && 1 == p.status {
					fmt.Println("start ", net, " handle failed, ", err)

					p.interrupt <- syscall.SIGTERM
				}
			}(k, v)
		}
	}

	signal.Notify(p.interrupt, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for sig := range p.interrupt {
		switch sig {
		case os.Interrupt, syscall.SIGINT:
			if 0 == p.exitConfirm {
				fmt.Println("Send ^C to force exit...")
			}

			if p.exitConfirm > 0 {
				p.shutdown()
			}

			p.exitConfirm++
		case os.Kill, syscall.SIGTERM:
			p.shutdown()
		case syscall.SIGHUP:
			p.reload()
		}
	}

	return nil
}

// reload reload config file and reset query cache
func (p *Proxy) reload() {
	var err = p.service.Reload()
	if nil != err {
		fmt.Println("proxy: reload config failed," + err.Error())

		p.shutdown()
	}
}

// reset reset query cache
func (p *Proxy) reset() {
	p.service.Reset()
}

// shutdown 关闭 DNS 代理服务
func (p *Proxy) shutdown() {
	if 1 == p.status {
		p.status = 0
		for k, v := range p.provider {
			go func(net string, handle ReverseProxy) {
				fmt.Println("proxy: stoping ", net, " at socket ", p.service.config.Bind[net])

				var err = handle.Stop()
				if nil != err {
					fmt.Println("Stop ", net, " handle failed, ", err)
				}

				p.wg.Done()
			}(k, v)
		}
	}

	p.wg.Wait()
	p.service.Shutdown()

	close(p.interrupt)
}

// createPidFile create process id file
func (p *Proxy) createPidFile(pidPath string) error {
	var pid, err = os.OpenFile(pidPath, os.O_RDWR|os.O_CREATE, 0666)
	if nil == err {
		defer pid.Close()

		_, err = pid.Write([]byte(strconv.Itoa(syscall.Getpid())))
	}

	return err
}
