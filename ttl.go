package redis

import (
	"github.com/miekg/dns"
)

func msgTTL(m *dns.Msg, ttl int) {
	for i := range m.Answer {
		m.Answer[i].Header().Ttl = uint32(ttl)
	}
	for i := range m.Ns {
		m.Ns[i].Header().Ttl = uint32(ttl)
	}
	for i := range m.Extra {
		if m.Extra[i].Header().Rrtype != dns.TypeOPT {
			m.Extra[i].Header().Ttl = uint32(ttl)
		}
	}
}
