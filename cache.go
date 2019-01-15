package redis

import (
	"fmt"
	"net/url"
	"time"

	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Return key under which we store the message, -1 will be returned if we don't store the message.
// Currently we do not cache Truncated, errors, zone transfers or dynamic update messages.
func key(m *dns.Msg, t response.Type, do bool) string {
	// We don't store truncated responses.
	if m.Truncated {
		return ""
	}
	// Nor errors or Meta or Update
	if t == response.OtherError || t == response.Meta || t == response.Update {
		return ""
	}
	return hash(m.Question[0].Name, m.Question[0].Qtype, do)
}

func hash(qName string, qType uint16, do bool) string {
	return url.QueryEscape(fmt.Sprint(qName, "-", qType, "-", do))
}

// ResponseWriter is a response writer that caches the reply message in Redis.
type ResponseWriter struct {
	dns.ResponseWriter
	state request.Request
	*Redis
	server string
}

// WriteMsg implements the dns.ResponseWriter interface.
func (w *ResponseWriter) WriteMsg(res *dns.Msg) error {
	do := false
	mt, opt := response.Typify(res, w.now().UTC())
	if opt != nil {
		do = opt.Do()
	}

	// key returns empty string for anything we don't want to cache.
	key := key(res, mt, do)

	duration := w.MsgTTL(res, mt)

	if key != "" && duration > 0 {

		if w.state.Match(res) {
			w.set(res, key, mt, duration)
		} else {
			// Don't log it, but increment counter
			cacheDrops.WithLabelValues(w.server).Inc()
		}
	}

	// Apply capped TTL to this reply to avoid jarring TTL experience 1799 -> 8 (e.g.)
	ttl := uint32(duration.Seconds())
	for i := range res.Answer {
		res.Answer[i].Header().Ttl = ttl
	}
	for i := range res.Ns {
		res.Ns[i].Header().Ttl = ttl
	}
	for i := range res.Extra {
		if res.Extra[i].Header().Rrtype != dns.TypeOPT {
			res.Extra[i].Header().Ttl = ttl
		}
	}
	return w.ResponseWriter.WriteMsg(res)
}

func (w *ResponseWriter) set(m *dns.Msg, key string, mt response.Type, duration time.Duration) {
	if key == "" || duration == 0 {
		return
	}

	switch mt {
	case response.NoError, response.Delegation:
		fallthrough

	case response.NameError, response.NoData:
		if err := Add(w.pool, key, m, duration); err != nil {
			log.Debugf("Failed to add response to Redis cache: %s", err)

			redisErr.WithLabelValues(w.server).Inc()
		}

	case response.OtherError:
		// don't cache these
	default:
		log.Warningf("Redis called with unknown typification: %d", mt)
	}
}

// Write implements the dns.ResponseWriter interface.
func (w *ResponseWriter) Write(buf []byte) (int, error) {
	log.Warningf("Redis called with Write: not caching reply")
	n, err := w.ResponseWriter.Write(buf)
	return n, err
}

func (w *ResponseWriter) MsgTTL(m *dns.Msg, mt response.Type) time.Duration {
	d := w.pttl
	if mt == response.NameError || mt == response.NoData {
		d = w.nttl
	}
	if mt != response.NoError && mt != response.NameError && mt != response.NoData {
		return 0
	}

	// No data to examine, return a short ttl as a fail safe.
	if len(m.Answer)+len(m.Ns) == 0 {
		return failSafeTTL
	}

	if w.normal {
		for _, r := range append(append(m.Answer, m.Ns...), m.Extra...) {
			if r.Header().Rrtype == dns.TypeOPT {
				// OPT records use TTL field for extended rcode and flags
				continue
			}
			switch mt {
			case response.NameError, response.NoData:
				if r.Header().Rrtype == dns.TypeSOA {
					return time.Duration(r.(*dns.SOA).Minttl) * time.Second
				}
			case response.NoError, response.Delegation:
				if r.Header().Ttl < uint32(d.Seconds()) {
					d = time.Duration(r.Header().Ttl) * time.Second
				}
			}
		}
	}
	return d
}

const (
	SuccessTTL  = 1 * time.Hour
	DenialTTL   = 30 * time.Minute
	failSafeTTL = 5 * time.Second

	// Success is the class for caching positive caching.
	Success = "success"
	// Denial is the class defined for negative caching.
	Denial = "denial"
)
