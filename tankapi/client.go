package tankapi

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type Client struct{}

func NewClient(args ...Timeouts) *Client {
	if len(args) > 0  {
		if args[0].DialTimeout != time.Duration(0) {
			timeout.DialTimeout = args[0].DialTimeout
		}
		if args[0].TlsHandshakeTimeout != time.Duration(0) {
			timeout.TlsHandshakeTimeout = args[0].TlsHandshakeTimeout
		}
		if args[0].NetClientTimeout != time.Duration(0) {
			timeout.NetClientTimeout = args[0].NetClientTimeout
		}
		if args[0].PrepareTimeout != time.Duration(0) {
			timeout.PrepareTimeout = args[0].PrepareTimeout
		}
		if args[0].PrepareAttemptsLimit != 0 {
			timeout.PrepareAttemptsLimit = args[0].PrepareAttemptsLimit
		}
	}

	netTransport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: timeout.DialTimeout,
		}).DialContext,
		TLSHandshakeTimeout: timeout.TlsHandshakeTimeout,
	}

	netClient = &http.Client{
		Transport: netTransport,
		Timeout:   timeout.NetClientTimeout,
	}

	return &Client{}
}

// Close - does nothing. made for overall consistency
// May be it could cleanup sessions or something.
func (*Client) Close() error {
	return nil
}

// Validate - sends config into corresponding tank apis to validate them
func (*Client) Validate(sessions []*Session) error {
	c := make(chan error, len(sessions))
	var errs []string
	for _, s := range sessions {
		go func(s *Session, c chan<- error) {
			c <- s.validate()
		}(s, c)
	}
	for i := 0; i < len(sessions); i++ {
		err := <-c
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// Prepare - starts tankapi sessions with breakpoint set to "run", so that tanks will prepare to be started
// ??? validate before preparing ???
func (*Client) Prepare(sessions []*Session) error {
	c := make(chan error, len(sessions))
	var errs []string
	for _, s := range sessions {
		go func(s *Session, c chan<- error) {
			c <- s.prepare()
		}(s, c)
	}
	for i := 0; i < len(sessions); i++ {
		err := <-c
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// Run - sets tankapi sessions breakpoint to "finished", so that tanks will run at once
func (*Client) Run(sessions []*Session) error {
	c := make(chan error, len(sessions))
	var errs []string
	for _, s := range sessions {
		go func(s *Session, c chan<- error) {
			c <- s.run()
		}(s, c)
	}
	for i := 0; i < len(sessions); i++ {
		err := <-c
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// Stop - stops tankapi sessions
func (*Client) Stop(sessions []*Session) error {
	c := make(chan error, len(sessions))
	var errs []string
	for _, s := range sessions {
		go func(s *Session, c chan<- error) {
			c <- s.stop()
		}(s, c)
	}
	for i := 0; i < len(sessions); i++ {
		err := <-c
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// Poll - polls tankapi sessions' status
func (*Client) Poll(sessions []*Session) error {
	c := make(chan error, len(sessions))
	var errs []string
	for _, s := range sessions {
		go func(s *Session, c chan<- error) {
			c <- s.poll()
		}(s, c)
	}
	for i := 0; i < len(sessions); i++ {
		err := <-c
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func (*Client) GetTimeouts() Timeouts {
	return timeout
}