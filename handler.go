package redis

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// ServeDNS implements the plugin.Handler interface.
func (r *Redis) ServeDNS(ctx context.Context, w dns.ResponseWriter, msg *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: msg}

	zone := plugin.Zones(r.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(r.Name(), r.Next, ctx, w, msg)
	}

	server := metrics.WithServer(ctx)
	now := r.now().UTC()

	m := r.get(now, state, server)
	if m == nil {
		crr := &ResponseWriter{ResponseWriter: w, Redis: r, state: state, server: metrics.WithServer(ctx)}
		return plugin.NextOrFailure(r.Name(), r.Next, ctx, crr, msg)
	}

	m.SetReply(msg)
	state.SizeAndDo(m)
	m = state.Scrub(m)
	_ = w.WriteMsg(m)

	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (r *Redis) Name() string { return "redisc" }
