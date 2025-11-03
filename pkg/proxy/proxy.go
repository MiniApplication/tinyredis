package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hsn0918/tinyredis/pkg/RESP"
)

var (
	infoReplicationCommand = RESP.MakeArrayData([]RESP.RedisData{
		RESP.MakeBulkData([]byte("INFO")),
		RESP.MakeBulkData([]byte("replication")),
	}).ToBytes()

	errNotLeader   = errors.New("target is not leader")
	errClientClose = errors.New("client disconnected")
)

// Proxy routes incoming RESP connections to the current Raft leader. It probes
// the configured node addresses using `INFO replication` to discover leaders.
type Proxy struct {
	ListenAddr    string
	Targets       []string
	DialTimeout   time.Duration
	RetryInterval time.Duration
	Logger        *log.Logger

	mu            sync.RWMutex
	currentLeader string
	targets       []string
}

// Run starts the proxy and blocks until the context is cancelled or a fatal
// error occurs.
func (p *Proxy) Run(ctx context.Context) error {
	if p.ListenAddr == "" {
		return errors.New("proxy listen address must be set")
	}

	p.targets = sanitizeTargets(p.Targets)
	if len(p.targets) == 0 {
		return errors.New("proxy targets must be provided")
	}
	p.normalize()

	listener, err := net.Listen("tcp", p.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", p.ListenAddr, err)
	}
	defer listener.Close()

	p.logf("proxy listening on %s; targets=%s", listener.Addr().String(), strings.Join(p.targets, ","))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	var wg sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				wg.Wait()
				return nil
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return fmt.Errorf("accept connection: %w", err)
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			p.handleClient(ctx, c)
		}(conn)
	}
}

func (p *Proxy) handleClient(ctx context.Context, client net.Conn) {
	defer func() {
		_ = client.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		leader, err := p.discoverLeader(ctx)
		if err != nil {
			p.logf("discover leader failed: %v", err)
			if !p.wait(ctx, p.RetryInterval) {
				return
			}
			continue
		}

		upstream, err := p.dialLeader(ctx, leader)
		if err != nil {
			p.logf("dial leader %s failed: %v", leader, err)
			p.clearCurrentLeader(leader)
			if !p.wait(ctx, p.RetryInterval) {
				return
			}
			continue
		}

		err = p.pipeConnections(ctx, client, upstream)
		_ = upstream.Close()
		if err == nil || errors.Is(err, errClientClose) {
			return
		}

		p.clearCurrentLeader(leader)
		if !p.wait(ctx, p.RetryInterval) {
			return
		}
	}
}

func (p *Proxy) pipeConnections(ctx context.Context, client, upstream net.Conn) error {
	errCh := make(chan proxyError, 2)
	go proxyCopy(upstream, client, true, errCh)
	go proxyCopy(client, upstream, false, errCh)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err.err == nil || errors.Is(err.err, io.EOF) {
			if err.fromClient {
				return errClientClose
			}
			return nil
		}
		if err.fromClient {
			return errClientClose
		}
		return err.err
	}
}

func proxyCopy(dst net.Conn, src net.Conn, fromClient bool, ch chan<- proxyError) {
	_, err := io.Copy(dst, src)
	ch <- proxyError{fromClient: fromClient, err: err}
}

type proxyError struct {
	fromClient bool
	err        error
}

func (p *Proxy) discoverLeader(ctx context.Context) (string, error) {
	if current := p.getCurrentLeader(); current != "" {
		if leader, err := p.probeTarget(ctx, current); err == nil {
			return leader, nil
		}
	}

	var lastErr error
	for _, target := range p.targets {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		leader, err := p.probeTarget(ctx, target)
		if err == nil {
			p.setCurrentLeader(leader)
			return leader, nil
		}
		if !errors.Is(err, errNotLeader) {
			lastErr = err
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("no leader discovered")
}

func (p *Proxy) probeTarget(ctx context.Context, addr string) (string, error) {
	conn, err := net.DialTimeout("tcp", addr, p.DialTimeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(p.DialTimeout)); err != nil {
		return "", err
	}
	if _, err := conn.Write(infoReplicationCommand); err != nil {
		return "", err
	}

	resultCh := RESP.ParseStream(conn)
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res, ok := <-resultCh:
		if !ok {
			return "", errors.New("no response from target")
		}
		if res.Err != nil {
			return "", res.Err
		}
		bulk, ok := res.Data.(*RESP.BulkData)
		if !ok {
			return "", errors.New("unexpected response type from target")
		}
		payload := string(bulk.ByteData())
		if strings.Contains(payload, "role:master") {
			return addr, nil
		}
		return "", errNotLeader
	}
}

func (p *Proxy) dialLeader(ctx context.Context, addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, p.DialTimeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (p *Proxy) normalize() {
	if p.DialTimeout <= 0 {
		p.DialTimeout = 2 * time.Second
	}
	if p.RetryInterval <= 0 {
		p.RetryInterval = 500 * time.Millisecond
	}
	if p.Logger == nil {
		p.Logger = log.New(os.Stdout, "[failover-proxy] ", log.LstdFlags)
	}
}

func (p *Proxy) wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (p *Proxy) setCurrentLeader(addr string) {
	p.mu.Lock()
	p.currentLeader = addr
	p.mu.Unlock()
}

func (p *Proxy) getCurrentLeader() string {
	p.mu.RLock()
	addr := p.currentLeader
	p.mu.RUnlock()
	return addr
}

func (p *Proxy) clearCurrentLeader(addr string) {
	p.mu.Lock()
	if p.currentLeader == addr {
		p.currentLeader = ""
	}
	p.mu.Unlock()
}

func (p *Proxy) logf(format string, args ...interface{}) {
	if p.Logger != nil {
		p.Logger.Printf(format, args...)
		return
	}
	fmt.Printf(format+"\n", args...)
}

func sanitizeTargets(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{})
	for _, t := range in {
		clean := strings.TrimSpace(t)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}
