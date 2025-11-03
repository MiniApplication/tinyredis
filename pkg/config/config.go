package config

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var Configures *Config

func init() {

}

var (
	DefaultHost                  = "0.0.0.0"
	DefaultPort                  = 6379
	DefaultLogDir                = "./"
	DefaultLogLevel              = "info"
	DefaultLogSamplingEnabled    = true
	DefaultLogSamplingInterval   = time.Second
	DefaultLogSamplingInitial    = 100
	DefaultLogSamplingThereafter = 100
	DefaultShardNum              = 1024
	DefaultNodeID                = "node-1"
	DefaultRaftDir               = "./raft"
	DefaultRaftBind              = "127.0.0.1:7000"
	DefaultRaftHTTPAddr          = "127.0.0.1:17000"
	DefaultRaftJoinAddr          = ""
	DefaultRaftConnectionPool    = 3
	DefaultRaftTimeout           = 10 * time.Second
	DefaultRaftSnapshotThresh    = 10000
	DefaultRaftSnapshotIntvl     = 2 * time.Minute
)

type Config struct {
	ConfFile              string
	Host                  string
	Port                  int
	LogDir                string
	LogLevel              string
	LogSamplingEnabled    bool
	LogSamplingInterval   time.Duration
	LogSamplingInitial    int
	LogSamplingThereafter int
	ShardNum              int
	NodeID                string

	RaftDir               string
	RaftBind              string
	RaftHTTPAddr          string
	JoinAddr              string
	JoinAddrs             []string
	RaftBootstrap         bool
	RaftConnectionPool    int
	RaftTimeout           time.Duration
	RaftSnapshotThreshold uint64
	RaftSnapshotInterval  time.Duration
}
type CfgError struct {
	message string
}

func (err *CfgError) Error() string {
	return err.message
}

func Setup(cmd *cobra.Command) (*Config, error) {
	cfg := &Config{
		Host:                  DefaultHost,
		Port:                  DefaultPort,
		LogDir:                DefaultLogDir,
		LogLevel:              DefaultLogLevel,
		LogSamplingEnabled:    DefaultLogSamplingEnabled,
		LogSamplingInterval:   DefaultLogSamplingInterval,
		LogSamplingInitial:    DefaultLogSamplingInitial,
		LogSamplingThereafter: DefaultLogSamplingThereafter,
		ShardNum:              DefaultShardNum,
		NodeID:                DefaultNodeID,
		RaftDir:               DefaultRaftDir,
		RaftBind:              DefaultRaftBind,
		RaftHTTPAddr:          DefaultRaftHTTPAddr,
		JoinAddr:              DefaultRaftJoinAddr,
		RaftBootstrap:         false,
		RaftConnectionPool:    DefaultRaftConnectionPool,
		RaftTimeout:           DefaultRaftTimeout,
		RaftSnapshotThreshold: uint64(DefaultRaftSnapshotThresh),
		RaftSnapshotInterval:  DefaultRaftSnapshotIntvl,
	}
	var err error
	if err = cmd.ParseFlags(os.Args[1:]); err != nil {
		return nil, err
	}
	if cfg.Host, err = cmd.Flags().GetString("host"); err != nil {
		return nil, fmt.Errorf("failed to parse host flag: %w", err)
	}
	if cfg.Port, err = cmd.Flags().GetInt("port"); err != nil {
		return nil, fmt.Errorf("failed to parse port flag: %w", err)
	}
	if cfg.LogDir, err = cmd.Flags().GetString("logdir"); err != nil {
		return nil, fmt.Errorf("failed to parse logdir flag: %w", err)
	}
	if cfg.LogLevel, err = cmd.Flags().GetString("loglevel"); err != nil {
		return nil, fmt.Errorf("failed to parse loglevel flag: %w", err)
	}
	if cfg.LogSamplingEnabled, err = cmd.Flags().GetBool("log-sampling"); err != nil {
		return nil, fmt.Errorf("failed to parse log-sampling flag: %w", err)
	}
	if cfg.LogSamplingInterval, err = cmd.Flags().GetDuration("log-sampling-interval"); err != nil {
		return nil, fmt.Errorf("failed to parse log-sampling-interval flag: %w", err)
	}
	if cfg.LogSamplingInitial, err = cmd.Flags().GetInt("log-sampling-initial"); err != nil {
		return nil, fmt.Errorf("failed to parse log-sampling-initial flag: %w", err)
	}
	if cfg.LogSamplingThereafter, err = cmd.Flags().GetInt("log-sampling-thereafter"); err != nil {
		return nil, fmt.Errorf("failed to parse log-sampling-thereafter flag: %w", err)
	}
	if cfg.ShardNum, err = cmd.Flags().GetInt("shardnum"); err != nil {
		return nil, fmt.Errorf("failed to parse shardnum flag: %w", err)
	}
	if cfg.NodeID, err = cmd.Flags().GetString("node-id"); err != nil {
		return nil, fmt.Errorf("failed to parse node-id flag: %w", err)
	}
	if cfg.RaftDir, err = cmd.Flags().GetString("raft-dir"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-dir flag: %w", err)
	}
	if cfg.RaftBind, err = cmd.Flags().GetString("raft-bind"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-bind flag: %w", err)
	}
	if cfg.RaftHTTPAddr, err = cmd.Flags().GetString("raft-http"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-http flag: %w", err)
	}
	if cfg.JoinAddr, err = cmd.Flags().GetString("raft-join"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-join flag: %w", err)
	}
	cfg.JoinAddrs = ParseJoinAddrs(cfg.JoinAddr)
	if cfg.RaftBootstrap, err = cmd.Flags().GetBool("raft-bootstrap"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-bootstrap flag: %w", err)
	}
	if cfg.RaftConnectionPool, err = cmd.Flags().GetInt("raft-pool"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-pool flag: %w", err)
	}
	if cfg.RaftTimeout, err = cmd.Flags().GetDuration("raft-timeout"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-timeout flag: %w", err)
	}
	snapThresh, err := cmd.Flags().GetUint64("raft-snap-threshold")
	if err != nil {
		return nil, fmt.Errorf("failed to parse raft-snap-threshold flag: %w", err)
	}
	if snapThresh > 0 {
		cfg.RaftSnapshotThreshold = snapThresh
	}
	if cfg.RaftSnapshotInterval, err = cmd.Flags().GetDuration("raft-snap-interval"); err != nil {
		return nil, fmt.Errorf("failed to parse raft-snap-interval flag: %w", err)
	}
	Configures = cfg
	return cfg, nil
}

func (cfg *Config) Parse(cfgFile string) error {
	fl, err := os.Open(cfgFile)
	if err != nil {
		return err
	}

	defer func() {
		err := fl.Close()
		if err != nil {
			fmt.Printf("Close config file error: %s \n", err.Error())
		}
	}()

	reader := bufio.NewReader(fl)
	for {
		line, ioErr := reader.ReadString('\n')
		if ioErr != nil && ioErr != io.EOF {
			return ioErr
		}

		if len(line) > 0 && line[0] == '#' {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			cfgName := strings.ToLower(fields[0])
			if cfgName == "host" {
				if ip := net.ParseIP(fields[1]); ip == nil {
					ipErr := &CfgError{
						message: fmt.Sprintf("Given ip address %s is invalid", cfg.Host),
					}
					return ipErr
				}
				cfg.Host = fields[1]
			} else if cfgName == "port" {
				port, err := strconv.Atoi(fields[1])
				if err != nil {
					return err
				}
				if port <= 1024 || port >= 65535 {
					portErr := &CfgError{
						message: fmt.Sprintf("Listening port should be between 1024 and 65535, but %d is given.", port),
					}
					return portErr
				}
				cfg.Port = port
			} else if cfgName == "logdir" {
				cfg.LogDir = strings.ToLower(fields[1])
			} else if cfgName == "loglevel" {
				cfg.LogLevel = strings.ToLower(fields[1])
			} else if cfgName == "logsampling" {
				parsed, err := strconv.ParseBool(fields[1])
				if err != nil {
					return err
				}
				cfg.LogSamplingEnabled = parsed
			} else if cfgName == "logsamplinginterval" {
				cfg.LogSamplingInterval, err = time.ParseDuration(fields[1])
				if err != nil {
					return err
				}
			} else if cfgName == "logsamplinginitial" {
				value, err := strconv.Atoi(fields[1])
				if err != nil {
					return err
				}
				cfg.LogSamplingInitial = value
			} else if cfgName == "logsamplingthereafter" {
				value, err := strconv.Atoi(fields[1])
				if err != nil {
					return err
				}
				cfg.LogSamplingThereafter = value
			} else if cfgName == "shardnum" {
				cfg.ShardNum, err = strconv.Atoi(fields[1])
				if err != nil {
					fmt.Println("ShardNum should be a number. Get: ", fields[1])
					panic(err)
				}
			} else if cfgName == "nodeid" {
				cfg.NodeID = fields[1]
			} else if cfgName == "raftdir" {
				cfg.RaftDir = fields[1]
			} else if cfgName == "raftbind" {
				cfg.RaftBind = fields[1]
			} else if cfgName == "rafthttp" {
				cfg.RaftHTTPAddr = fields[1]
			} else if cfgName == "raftjoin" {
				cfg.JoinAddr = fields[1]
				cfg.JoinAddrs = ParseJoinAddrs(cfg.JoinAddr)
			} else if cfgName == "raftbootstrap" {
				v, parseErr := strconv.ParseBool(fields[1])
				if parseErr != nil {
					return parseErr
				}
				cfg.RaftBootstrap = v
			} else if cfgName == "raftpool" {
				cfg.RaftConnectionPool, err = strconv.Atoi(fields[1])
				if err != nil {
					return err
				}
			} else if cfgName == "rafttimeout" {
				cfg.RaftTimeout, err = time.ParseDuration(fields[1])
				if err != nil {
					return err
				}
			} else if cfgName == "raftsnapthreshold" {
				cfg.RaftSnapshotThreshold, err = strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return err
				}
			} else if cfgName == "raftsnapinterval" {
				cfg.RaftSnapshotInterval, err = time.ParseDuration(fields[1])
				if err != nil {
					return err
				}
			}
		}
		if ioErr == io.EOF {
			break
		}
	}
	return nil
}

func NewDefaultConfig() *Config {
	return &Config{
		Host:                  DefaultHost,
		Port:                  DefaultPort,
		LogDir:                DefaultLogDir,
		LogLevel:              DefaultLogLevel,
		LogSamplingEnabled:    DefaultLogSamplingEnabled,
		LogSamplingInterval:   DefaultLogSamplingInterval,
		LogSamplingInitial:    DefaultLogSamplingInitial,
		LogSamplingThereafter: DefaultLogSamplingThereafter,
		ShardNum:              DefaultShardNum,
		NodeID:                DefaultNodeID,
		RaftDir:               DefaultRaftDir,
		RaftBind:              DefaultRaftBind,
		RaftHTTPAddr:          DefaultRaftHTTPAddr,
		JoinAddr:              DefaultRaftJoinAddr,
		JoinAddrs:             nil,
		RaftBootstrap:         false,
		RaftConnectionPool:    DefaultRaftConnectionPool,
		RaftTimeout:           DefaultRaftTimeout,
		RaftSnapshotThreshold: uint64(DefaultRaftSnapshotThresh),
		RaftSnapshotInterval:  DefaultRaftSnapshotIntvl,
	}
}

func ParseJoinAddrs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\n':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
