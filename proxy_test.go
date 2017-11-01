package main

import (
	"testing"

	"github.com/miekg/dns"
)

const (
	nameserver = "127.0.0.1:53"
	domain     = "www.imohe.com"
)

func BenchmarkDig(b *testing.B) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	c := new(dns.Client)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.Exchange(m, nameserver)
	}
}
