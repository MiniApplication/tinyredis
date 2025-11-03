package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/config"
	"github.com/hsn0918/tinyredis/pkg/logger"
	"github.com/hsn0918/tinyredis/pkg/memdb"
)

const (
	applyTimeout        = 5 * time.Second
	httpJoinPath        = "/join"
	defaultHTTPRead     = 5 * time.Second
	joinRequestTimeout  = 5 * time.Second
	leaderCacheFileName = "leader.addr"
	leaderPollInterval  = 2 * time.Second
)

// Node wraps a hashicorp/raft instance together with the database FSM.
type Node struct {
	cfg        *config.Config
	raft       *raft.Raft
	db         *memdb.MemDb
	httpServer *http.Server
	httpAddr   string
	mu         sync.RWMutex
	leaderStop chan struct{}
	leaderWG   sync.WaitGroup
	leaderPath string
	logWriter  *raftLogWriter
}

type raftLogWriter struct {
	inner io.Writer
	mu    sync.RWMutex
	hint  func() (string, string)
}

func newRaftLogWriter(inner io.Writer) io.Writer {
	if inner == nil {
		inner = io.Discard
	}
	return &raftLogWriter{inner: inner}
}

func (w *raftLogWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("Rollback failed: tx closed")) {
		logger.Debug(strings.TrimSpace(string(p)))
		return len(p), nil
	}

	if bytes.Contains(p, []byte("leader-address=")) && bytes.Contains(p, []byte("leader-id=")) {
		addr, id := w.currentHint()
		if addr != "" || id != "" {
			s := string(p)
			if addr != "" {
				s = w.fillPlaceholder(s, "leader-address", addr)
			}
			if id != "" {
				s = w.fillPlaceholder(s, "leader-id", id)
			}
			p = []byte(s)
		}
	}

	return w.inner.Write(p)
}

func (w *raftLogWriter) fillPlaceholder(s, key, value string) string {
	if value == "" {
		return s
	}
	target := key + "="
	idx := strings.Index(s, target)
	if idx == -1 {
		return s
	}
	start := idx + len(target)
	if start >= len(s) {
		return s + value
	}
	switch s[start] {
	case ' ', '\n', '\r', '\t':
		return s[:start] + value + s[start:]
	default:
		return s
	}
}

func (w *raftLogWriter) SetLeaderHintFunc(fn func() (string, string)) {
	w.mu.Lock()
	w.hint = fn
	w.mu.Unlock()
}

func (w *raftLogWriter) currentHint() (string, string) {
	w.mu.RLock()
	fn := w.hint
	w.mu.RUnlock()
	if fn == nil {
		return "", ""
	}
	return fn()
}

// NewNode constructs and starts a Raft node using the provided configuration.
func NewNode(cfg *config.Config) (*Node, error) {
	if cfg.NodeID == "" {
		return nil, errors.New("config NodeID must be set")
	}
	if cfg.RaftDir == "" {
		return nil, errors.New("config RaftDir must be set")
	}
	if cfg.RaftBind == "" {
		return nil, errors.New("config RaftBind must be set")
	}
	if len(cfg.JoinAddrs) == 0 && strings.TrimSpace(cfg.JoinAddr) != "" {
		cfg.JoinAddrs = config.ParseJoinAddrs(cfg.JoinAddr)
	}

	poolSize := cfg.RaftConnectionPool
	if poolSize <= 0 {
		poolSize = config.DefaultRaftConnectionPool
	}
	timeout := cfg.RaftTimeout
	if timeout <= 0 {
		timeout = config.DefaultRaftTimeout
	}
	snapThreshold := cfg.RaftSnapshotThreshold
	if snapThreshold == 0 {
		snapThreshold = uint64(config.DefaultRaftSnapshotThresh)
	}
	snapInterval := cfg.RaftSnapshotInterval
	if snapInterval <= 0 {
		snapInterval = config.DefaultRaftSnapshotIntvl
	}

	db := memdb.NewMemDb()

	if err := os.MkdirAll(cfg.RaftDir, 0o755); err != nil {
		return nil, fmt.Errorf("create raft dir: %w", err)
	}

	logStorePath := filepath.Join(cfg.RaftDir, "raft-log.bolt")
	stableStorePath := filepath.Join(cfg.RaftDir, "raft-stable.bolt")

	logStore, err := raftboltdb.NewBoltStore(logStorePath)
	if err != nil {
		return nil, fmt.Errorf("new bolt log store: %w", err)
	}
	stableStore, err := raftboltdb.NewBoltStore(stableStorePath)
	if err != nil {
		return nil, fmt.Errorf("new bolt stable store: %w", err)
	}

	logWriter := newRaftLogWriter(os.Stderr)

	snapshotStore, err := raft.NewFileSnapshotStore(cfg.RaftDir, 2, logWriter)
	if err != nil {
		return nil, fmt.Errorf("new snapshot store: %w", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", cfg.RaftBind)
	if err != nil {
		return nil, fmt.Errorf("resolve raft bind %s: %w", cfg.RaftBind, err)
	}

	transport, err := raft.NewTCPTransport(cfg.RaftBind, addr, poolSize, timeout, logWriter)
	if err != nil {
		return nil, fmt.Errorf("new tcp transport: %w", err)
	}

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(cfg.NodeID)
	raftConfig.SnapshotThreshold = snapThreshold
	raftConfig.SnapshotInterval = snapInterval

	fsm := &fsm{db: db}

	r, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("new raft: %w", err)
	}

	hasState, err := raft.HasExistingState(logStore, stableStore, snapshotStore)
	if err != nil {
		return nil, fmt.Errorf("check existing raft state: %w", err)
	}

	if hasState {
		if cfg.RaftBootstrap {
			return nil, fmt.Errorf("found existing raft state under %s, remove --raft-bootstrap", cfg.RaftDir)
		}
		if len(cfg.JoinAddrs) > 0 || strings.TrimSpace(cfg.JoinAddr) != "" {
			return nil, fmt.Errorf("found existing raft state under %s, remove --raft-join", cfg.RaftDir)
		}
	}

	if cfg.RaftBootstrap && !hasState {
		cfg := raft.Configuration{
			Servers: []raft.Server{
				{
					Suffrage: raft.Voter,
					ID:       raftConfig.LocalID,
					Address:  transport.LocalAddr(),
				},
			},
		}
		future := r.BootstrapCluster(cfg)
		if err := future.Error(); err != nil && !errors.Is(err, raft.ErrCantBootstrap) {
			return nil, fmt.Errorf("bootstrap cluster: %w", err)
		}
	}

	node := &Node{
		cfg:        cfg,
		raft:       r,
		db:         db,
		leaderStop: make(chan struct{}),
		leaderPath: filepath.Join(cfg.RaftDir, leaderCacheFileName),
		logWriter:  logWriter.(*raftLogWriter),
	}

	node.logWriter.SetLeaderHintFunc(node.leaderHint)
	db.SetReplicationInfoFetcher(node.collectReplicationInfo)
	node.startLeaderWatcher()

	if cfg.RaftHTTPAddr != "" {
		if err := node.startHTTPServer(cfg.RaftHTTPAddr); err != nil {
			return nil, fmt.Errorf("start raft http server: %w", err)
		}
	}

	if err := node.maybeJoinCluster(hasState, string(transport.LocalAddr())); err != nil {
		return nil, err
	}

	return node, nil
}

// ApplyCommand replicates a write command through Raft and waits for application.
func (n *Node) ApplyCommand(cmd [][]byte) ([]byte, error) {
	if n.raft.State() != raft.Leader {
		leaderAddr, leaderID := n.raft.LeaderWithID()
		return nil, fmt.Errorf("not leader (leader=%s id=%s)", leaderAddr, leaderID)
	}
	data, err := encodeCommand(cmd)
	if err != nil {
		return nil, err
	}
	future := n.raft.Apply(data, applyTimeout)
	if err := future.Error(); err != nil {
		return nil, err
	}
	resp, ok := future.Response().([]byte)
	if !ok {
		return nil, errors.New("invalid response from raft apply")
	}
	return resp, nil
}

// ReadCommand executes a read-only command against the local state machine.
func (n *Node) ReadCommand(cmd [][]byte) ([]byte, error) {
	allowLocal := len(cmd) > 0 && strings.EqualFold(string(cmd[0]), "info")
	if !allowLocal && n.raft.State() != raft.Leader {
		leaderAddr, leaderID := n.raft.LeaderWithID()
		return nil, fmt.Errorf("not leader (leader=%s id=%s)", leaderAddr, leaderID)
	}

	res := n.db.ExecCommand(cmd)
	if res == nil {
		return RESP.MakeErrorData("unknown error").ToBytes(), nil
	}
	return res.ToBytes(), nil
}

// DB exposes the underlying MemDb for diagnostics/tests.
func (n *Node) DB() *memdb.MemDb {
	return n.db
}

// Stop shuts down the raft node and HTTP join server.
func (n *Node) Stop() error {
	var errs []string

	if n.leaderStop != nil {
		close(n.leaderStop)
		n.leaderWG.Wait()
		n.leaderStop = nil
	}

	if n.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := n.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if n.raft != nil {
		future := n.raft.Shutdown()
		if err := future.Error(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("stop raft node: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (n *Node) startLeaderWatcher() {
	if n.leaderStop == nil || n.leaderPath == "" || n.raft == nil {
		return
	}
	n.leaderWG.Add(1)
	go n.leaderWatchLoop()
}

func (n *Node) leaderWatchLoop() {
	defer n.leaderWG.Done()
	ticker := time.NewTicker(leaderPollInterval)
	defer ticker.Stop()

	lastAddr := strings.TrimSpace(n.cachedLeaderAddr())

	for {
		select {
		case <-ticker.C:
			addr, _ := n.raft.LeaderWithID()
			current := strings.TrimSpace(string(addr))
			if current == "" || current == lastAddr {
				continue
			}
			if err := n.persistLeaderAddr(current); err != nil {
				logger.Debugf("persist leader addr: %v", err)
				continue
			}
			lastAddr = current
		case <-n.leaderStop:
			return
		}
	}
}

func (n *Node) persistLeaderAddr(addr string) error {
	if n.leaderPath == "" || addr == "" {
		return nil
	}
	return os.WriteFile(n.leaderPath, []byte(addr), 0o600)
}

func (n *Node) cachedLeaderAddr() string {
	if n.leaderPath == "" {
		return ""
	}
	data, err := os.ReadFile(n.leaderPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Debugf("read leader cache: %v", err)
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (n *Node) leaderHint() (string, string) {
	if n == nil || n.raft == nil {
		return "", ""
	}
	addr, id := n.raft.LeaderWithID()
	leaderAddr := strings.TrimSpace(string(addr))
	if leaderAddr == "" {
		leaderAddr = n.cachedLeaderAddr()
		if leaderAddr == "" && len(n.cfg.JoinAddrs) > 0 {
			leaderAddr = strings.TrimSpace(n.cfg.JoinAddrs[0])
		}
	}
	return leaderAddr, strings.TrimSpace(string(id))
}

func (n *Node) collectReplicationInfo() RESP.RedisData {
	builder := &strings.Builder{}
	builder.WriteString("# Replication\n")

	if n == nil || n.raft == nil {
		builder.WriteString("role:unknown\n")
		builder.WriteString("detail:raft_not_initialized\n")
		return RESP.MakeBulkData([]byte(builder.String()))
	}

	role := "unknown"
	state := n.raft.State()
	switch state {
	case raft.Leader:
		role = "master"
	case raft.Follower, raft.Candidate:
		role = "slave"
	case raft.Shutdown:
		role = "shutdown"
	}
	builder.WriteString(fmt.Sprintf("role:%s\n", role))

	if n.cfg != nil && n.cfg.NodeID != "" {
		builder.WriteString(fmt.Sprintf("node_id:%s\n", n.cfg.NodeID))
	}

	if n.cfg != nil && n.cfg.Host != "" && n.cfg.Port != 0 {
		builder.WriteString(fmt.Sprintf("advertise_addr:%s\n", net.JoinHostPort(n.cfg.Host, strconv.Itoa(n.cfg.Port))))
	}

	builder.WriteString(fmt.Sprintf("raft_state:%s\n", state.String()))
	builder.WriteString(fmt.Sprintf("raft_state_id:%d\n", uint32(state)))
	builder.WriteString(fmt.Sprintf("raft_term:%d\n", n.raft.CurrentTerm()))
	builder.WriteString(fmt.Sprintf("raft_commit_index:%d\n", n.raft.CommitIndex()))
	builder.WriteString(fmt.Sprintf("raft_applied_index:%d\n", n.raft.AppliedIndex()))
	builder.WriteString(fmt.Sprintf("raft_last_log_index:%d\n", n.raft.LastIndex()))

	leaderAddr, leaderID := n.raft.LeaderWithID()
	if len(leaderAddr) > 0 || len(leaderID) > 0 {
		builder.WriteString(fmt.Sprintf("leader_id:%s\n", strings.TrimSpace(string(leaderID))))
		builder.WriteString(fmt.Sprintf("leader_raft_addr:%s\n", strings.TrimSpace(string(leaderAddr))))
	} else {
		builder.WriteString("leader_id:\n")
	}

	if leader := strings.TrimSpace(string(leaderID)); leader != "" && n.cfg != nil && leader == n.cfg.NodeID {
		if httpAddr := n.HTTPAddr(); httpAddr != "" {
			builder.WriteString(fmt.Sprintf("leader_http_addr:%s\n", httpAddr))
		} else if n.cfg.RaftHTTPAddr != "" {
			builder.WriteString(fmt.Sprintf("leader_http_addr:%s\n", n.cfg.RaftHTTPAddr))
		}
	}

	cfgFuture := n.raft.GetConfiguration()
	if err := cfgFuture.Error(); err != nil {
		builder.WriteString(fmt.Sprintf("config_error:%s\n", err.Error()))
	} else {
		cfg := cfgFuture.Configuration()
		builder.WriteString(fmt.Sprintf("known_peers:%d\n", len(cfg.Servers)))
		for idx, srv := range cfg.Servers {
			builder.WriteString(fmt.Sprintf("peer%d:id=%s,raft_addr=%s,suffrage=%s,self=%t\n",
				idx,
				string(srv.ID),
				string(srv.Address),
				srv.Suffrage.String(),
				n.cfg != nil && string(srv.ID) == n.cfg.NodeID,
			))
		}
	}

	if strings.TrimSpace(string(leaderAddr)) == "" {
		if cached := n.cachedLeaderAddr(); cached != "" {
			builder.WriteString(fmt.Sprintf("leader_hint:%s\n", cached))
		} else {
			builder.WriteString("detail:leader_not_known\n")
		}
	}

	return RESP.MakeBulkData([]byte(builder.String()))
}

// LeaderID returns the current leader ID if known.
func (n *Node) LeaderID() string {
	_, id := n.raft.LeaderWithID()
	return string(id)
}

// ID returns the configured node identifier.
func (n *Node) ID() string {
	return n.cfg.NodeID
}

// RaftState exposes the current Raft state (Leader, Follower, etc.).
func (n *Node) RaftState() raft.RaftState {
	return n.raft.State()
}

// Barrier waits for all outstanding logs to be applied cluster-wide.
func (n *Node) Barrier(timeout time.Duration) error {
	return n.raft.Barrier(timeout).Error()
}

// HTTPAddr returns the advertise address for the join HTTP server.
func (n *Node) HTTPAddr() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.httpAddr
}

func (n *Node) maybeJoinCluster(hasState bool, raftAddr string) error {
	if hasState {
		return nil
	}
	targets := n.collectJoinTargets()
	if len(targets) == 0 {
		return nil
	}
	if err := n.join(targets, raftAddr); err != nil {
		return fmt.Errorf("join cluster: %w", err)
	}
	return nil
}

func (n *Node) collectJoinTargets() []string {
	seen := make(map[string]struct{})
	var targets []string

	push := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		key := normalizeJoinAddr(addr)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		targets = append(targets, addr)
	}

	if cached := n.cachedLeaderAddr(); cached != "" {
		push(cached)
	}
	for _, addr := range n.cfg.JoinAddrs {
		push(addr)
	}

	return targets
}

func (n *Node) startHTTPServer(bind string) error {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}

	n.mu.Lock()
	n.httpAddr = listener.Addr().String()
	n.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc(httpJoinPath, n.handleJoin)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: defaultHTTPRead,
	}
	n.httpServer = server

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("raft join http server error: ", err)
		}
	}()
	return nil
}

func (n *Node) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	nodeID := r.FormValue("id")
	addr := r.FormValue("addr")
	if nodeID == "" || addr == "" {
		http.Error(w, "missing id or addr", http.StatusBadRequest)
		return
	}

	future := n.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if err := future.Error(); err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "exists") || strings.Contains(lower, "known peer") {
			logger.Info("raft join request already satisfied for ", nodeID, " at ", addr)
			_, _ = w.Write([]byte("ok"))
			return
		}

		if errors.Is(err, raft.ErrNotLeader) || strings.Contains(lower, "not leader") {
			leaderAddr := ""
			if addrVal, _ := n.raft.LeaderWithID(); addrVal != "" {
				leaderAddr = strings.TrimSpace(string(addrVal))
			}
			if leaderAddr != "" {
				w.Header().Set("X-Raft-Leader", leaderAddr)
				w.Header().Set("Location", fmt.Sprintf("http://%s%s", leaderAddr, httpJoinPath))
			}
			http.Error(w, fmt.Sprintf("not leader (leader=%s)", leaderAddr), http.StatusTemporaryRedirect)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Info("accepted raft join request from ", nodeID, " at ", addr)
	_, _ = w.Write([]byte("ok"))
}

func (n *Node) join(targets []string, raftAddr string) error {
	if len(targets) == 0 {
		return nil
	}

	queue := append([]string(nil), targets...)
	seen := make(map[string]struct{}, len(targets))
	var lastErr error

	for len(queue) > 0 {
		target := strings.TrimSpace(queue[0])
		queue = queue[1:]

		if target == "" {
			continue
		}
		normalized := normalizeJoinAddr(target)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}

		hint, err := n.joinOnce(target, raftAddr)
		if err == nil {
			if hint != "" {
				if persistErr := n.persistLeaderAddr(hint); persistErr != nil {
					logger.Debugf("persist leader addr: %v", persistErr)
				}
			} else if persistErr := n.persistLeaderAddr(target); persistErr != nil {
				logger.Debugf("persist leader addr: %v", persistErr)
			}
			return nil
		}

		lastErr = err
		if hint != "" {
			hint = strings.TrimSpace(hint)
			hintKey := normalizeJoinAddr(hint)
			if hintKey != "" {
				if _, ok := seen[hintKey]; !ok {
					queue = append([]string{hint}, queue...)
				}
			}
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no join targets available")
}

func (n *Node) joinOnce(joinAddr, raftAddr string) (string, error) {
	joinURL, err := buildJoinURL(joinAddr)
	if err != nil {
		return "", fmt.Errorf("invalid join addr %q: %w", joinAddr, err)
	}

	form := url.Values{}
	form.Set("id", n.cfg.NodeID)
	form.Set("addr", raftAddr)

	req, err := http.NewRequest(http.MethodPost, joinURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build join request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: joinRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("join request to %s failed: %w", joinAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return strings.TrimSpace(joinAddr), nil
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))
	leaderHint := parseLeaderHint(resp, bodyStr)

	return leaderHint, fmt.Errorf("join request to %s rejected (status %d): %s", joinAddr, resp.StatusCode, bodyStr)
}

func buildJoinURL(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("empty join address")
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		u, err := url.Parse(addr)
		if err != nil {
			return "", err
		}
		if u.Path == "" || u.Path == "/" {
			u.Path = httpJoinPath
		} else {
			u.Path = strings.TrimSuffix(u.Path, "/") + httpJoinPath
		}
		return u.String(), nil
	}
	return fmt.Sprintf("http://%s%s", addr, httpJoinPath), nil
}

func parseLeaderHint(resp *http.Response, body string) string {
	if resp != nil {
		if hint := strings.TrimSpace(resp.Header.Get("X-Raft-Leader")); hint != "" {
			return hint
		}
		if loc := strings.TrimSpace(resp.Header.Get("Location")); loc != "" {
			// Location may point to the leader URL directly.
			return strings.TrimSuffix(loc, httpJoinPath)
		}
	}

	if body == "" {
		return ""
	}
	lower := strings.ToLower(body)
	idx := strings.Index(lower, "leader")
	if idx == -1 {
		return ""
	}
	suffix := body[idx:]
	sep := strings.IndexAny(suffix, "=:")
	if sep >= 0 && sep+1 < len(suffix) {
		suffix = suffix[sep+1:]
	}
	suffix = strings.TrimLeft(suffix, " \t")
	for i, r := range suffix {
		if r == ')' || r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' {
			suffix = suffix[:i]
			break
		}
	}
	return strings.Trim(suffix, "\"")
}

func normalizeJoinAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	addr = strings.TrimSuffix(addr, "/")
	if strings.HasPrefix(addr, "http://") {
		return strings.TrimPrefix(addr, "http://")
	}
	if strings.HasPrefix(addr, "https://") {
		return strings.TrimPrefix(addr, "https://")
	}
	return addr
}

type fsm struct {
	db *memdb.MemDb
}

func (f *fsm) Apply(log *raft.Log) interface{} {
	cmd, err := decodeCommand(log.Data)
	if err != nil {
		errResp := RESP.MakeErrorData("apply error: " + err.Error())
		return errResp.ToBytes()
	}
	res := f.db.ExecCommand(cmd)
	if res == nil {
		return RESP.MakeErrorData("unknown error").ToBytes()
	}
	return res.ToBytes()
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{}, nil
}

func (f *fsm) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	// No-op restore: rely on log replay.
	return nil
}

type fsmSnapshot struct{}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	defer sink.Close()
	return nil
}

func (s *fsmSnapshot) Release() {}

func encodeCommand(cmd [][]byte) ([]byte, error) {
	items := make([]RESP.RedisData, len(cmd))
	for i, arg := range cmd {
		items[i] = RESP.MakeBulkData(arg)
	}
	return RESP.MakeArrayData(items).ToBytes(), nil
}

func decodeCommand(data []byte) ([][]byte, error) {
	ch := RESP.ParseStream(bytes.NewReader(data))
	res, ok := <-ch
	if !ok {
		return nil, errors.New("empty raft log entry")
	}
	if res.Err != nil {
		return nil, res.Err
	}
	array, ok := res.Data.(*RESP.ArrayData)
	if !ok {
		return nil, errors.New("raft log entry is not array")
	}
	return array.ToCommand(), nil
}
