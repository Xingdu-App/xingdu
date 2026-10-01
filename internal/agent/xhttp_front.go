package agent

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go/http3"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"slices"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) { return c.r.Read(b) }

func serveXHTTP(ctx context.Context, s protocol.Spec, socket string) error {
	var tc *tls.Config
	if s.NeedsCertificate() {
		pair, err := tls.X509KeyPair([]byte(s.Certificate), []byte(s.PrivateKey))
		if err != nil {
			return err
		}
		tc = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}}
	}
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{Protocols: protocols, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.Out.URL.Scheme = "http"
		r.Out.URL.Host = "xingdu.internal"
		r.Out.Host = r.In.Host
	}, Transport: transport, FlushInterval: -1, ErrorLog: log.New(io.Discard, "", 0), ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) { http.Error(w, "upstream unavailable", 502) }}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enabled := s.TLSEnabled()
		alpn := s.V2Ray.ALPN
		if r.Method == http.MethodGet && s.V2Ray.Download != nil {
			enabled = s.V2Ray.Download.TLS
			alpn = s.V2Ray.Download.ALPN
		}
		if (r.TLS != nil) != enabled {
			http.Error(w, "transport mismatch", 400)
			return
		}
		if enabled && len(alpn) > 0 {
			actual := map[int]string{1: "http/1.1", 2: "h2", 3: "h3"}[r.ProtoMajor]
			if !slices.Contains(alpn, actual) {
				http.Error(w, "transport mismatch", 400)
				return
			}
		}
		proxy.ServeHTTP(w, r)
	})
	ps := new(http.Protocols)
	ps.SetHTTP1(true)
	ps.SetHTTP2(true)
	ps.SetUnencryptedHTTP2(true)
	server := &http.Server{Handler: handler, Protocols: ps, TLSConfig: tc, ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 65536, ErrorLog: log.New(io.Discard, "", 0)}
	raw, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	if err != nil {
		return err
	}
	defer raw.Close()
	mux := newDispatchListener(raw)
	defer mux.Close()
	defer server.Close()
	errors := make(chan error, 3)
	go func() { errors <- server.Serve(mux) }()
	go func() { errors <- mux.dispatch(ctx, tc) }()
	var h3 *http3.Server
	if tc != nil {
		h3 = &http3.Server{Addr: fmt.Sprintf(":%d", s.Port), TLSConfig: tc.Clone(), Handler: handler, MaxHeaderBytes: 65536}
		packet, err := net.ListenPacket("udp", h3.Addr)
		if err != nil {
			return err
		}
		defer packet.Close()
		defer h3.Close()
		go func() { errors <- h3.Serve(packet) }()
	}
	select {
	case <-ctx.Done():
		return nil
	case e := <-errors:
		return e
	}
}

// Dispatch a bounded number of protocol peeks concurrently. The HTTP server
// receives *tls.Conn for TLS (including ALPN) and a buffered plaintext connection
// otherwise, allowing upload/download TLS settings to share one XHTTP session.
type dispatchListener struct {
	net.Listener
	ready  chan net.Conn
	closed chan struct{}
	once   chan struct{}
}

func newDispatchListener(l net.Listener) *dispatchListener {
	return &dispatchListener{Listener: l, ready: make(chan net.Conn), closed: make(chan struct{}), once: make(chan struct{}, 1)}
}
func (l *dispatchListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ready:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}
func (l *dispatchListener) Close() error {
	select {
	case l.once <- struct{}{}:
		close(l.closed)
		return l.Listener.Close()
	default:
		return nil
	}
}
func (l *dispatchListener) dispatch(ctx context.Context, tc *tls.Config) error {
	slots := make(chan struct{}, 128)
	for {
		c, e := l.Listener.Accept()
		if e != nil {
			return e
		}
		select {
		case slots <- struct{}{}:
		default:
			c.Close()
			continue
		}
		go func() {
			defer func() { <-slots }()
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			r := bufio.NewReader(c)
			b, e := r.Peek(1)
			if e != nil {
				c.Close()
				return
			}
			c.SetReadDeadline(time.Time{})
			var conn net.Conn = &bufferedConn{c, r}
			if b[0] == 22 {
				if tc == nil {
					c.Close()
					return
				}
				conn = tls.Server(conn, tc)
			}
			select {
			case l.ready <- conn:
			case <-ctx.Done():
				conn.Close()
			case <-l.closed:
				conn.Close()
			}
		}()
	}
}
