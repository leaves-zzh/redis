package redis

import (
	"testing"
	"time"

	"github.com/mholt/caddy"
)

func TestSetup(t *testing.T) {
	const defEndpoint = "127.0.0.1:6379"

	tests := []struct {
		input            string
		shouldErr        bool
		expectedNTtl     time.Duration
		expectedPTtl     time.Duration
		expectedEndpoint string
	}{
		{`redis`, false, DenialTTL, SuccessTTL, defEndpoint},
		{`redis example.nl {
					success 10
				}`, false, DenialTTL, 10 * time.Second, defEndpoint},
		{`redis example.nl {
					success 10
					denial 15
				}`, false, 15 * time.Second, 10 * time.Second, defEndpoint},
		{`redis	{
				endpoint 127.0.0.2:6379
			}`, false, DenialTTL, SuccessTTL, "127.0.0.2:6379"},
		{`redis	{
				endpoint 127.0.0.3
			}`, false, DenialTTL, SuccessTTL, "127.0.0.3:6379"},

		// fails
		{`redis example.nl {
				success 15
				denial aaa
			}`, true, DenialTTL, SuccessTTL, defEndpoint},
		{`redis example.nl {
				positive 15
				negative aaa
			}`, true, DenialTTL, SuccessTTL, defEndpoint},
		{`redis {
				endpoint :1:1:6379
			}`, true, DenialTTL, SuccessTTL, defEndpoint},
		{`redis {
				endpoint 127.0.0.a
			}`, true, DenialTTL, SuccessTTL, defEndpoint},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		re, err := parse(c)
		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if re.nttl != test.expectedNTtl {
			t.Errorf("Test %v: Expected nttl %v but found: %v", i, test.expectedNTtl, re.nttl)
		}
		if re.pttl != test.expectedPTtl {
			t.Errorf("Test %v: Expected pttl %v but found: %v", i, test.expectedPTtl, re.pttl)
		}
		if re.addr != test.expectedEndpoint {
			t.Errorf("Test %v: Expected endpoint %v but found: %v", i, test.expectedEndpoint, re.addr)
		}
	}
}
