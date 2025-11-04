package cmd

import (
	"fmt"
	"os"

	"github.com/hsn0918/tinyredis/pkg/config"
	"github.com/hsn0918/tinyredis/pkg/logger"
	"github.com/hsn0918/tinyredis/pkg/memdb"
	"github.com/hsn0918/tinyredis/pkg/server"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "tinyredis",
	Short:         "A tiny Redis toolkit",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runNode(cmd)
		}
		return cmd.Help()
	},
}

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Run a tinyredis node",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNode(cmd)
	},
}

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

$ source <(tiny-redis completion bash)

# To load completions for each session, execute once:
Linux:
  $ tiny-redis completion bash > /etc/bash_completion.d/tiny-redis
MacOS:
  $ tiny-redis completion bash > /usr/local/etc/bash_completion.d/tiny-redis

Zsh:

$ source <(tiny-redis completion zsh)

# To load completions for each session, execute once:
$ tiny-redis completion zsh > "${fpath[1]}/_tiny-redis"

# You will need to start a new shell for this setup to take effect.

Fish:

$ tiny-redis completion fish | source

# To load completions for each session, execute once:
$ tiny-redis completion fish > ~/.config/fish/completions/tiny-redis.fish
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 && args[0] == "bash" {
			if err := rootCmd.GenBashCompletion(os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "generate bash completion:", err)
			}
		} else if len(args) > 0 && args[0] == "zsh" {
			if err := rootCmd.GenZshCompletion(os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "generate zsh completion:", err)
			}
		} else if len(args) > 0 && args[0] == "fish" {
			if err := rootCmd.GenFishCompletion(os.Stdout, true); err != nil {
				fmt.Fprintln(os.Stderr, "generate fish completion:", err)
			}
		} else {
			fmt.Println("Please specify a shell: bash, zsh, or fish")
		}
	},
}

func init() {
	config.Configures = config.NewDefaultConfig()
	bindNodeFlags(rootCmd)
	bindNodeFlags(nodeCmd)

	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(completionCmd)
	memdb.RegisterKeyCommand()
	memdb.RegisterStringCommands()
	memdb.RegisterHashCommands()
	memdb.RegisterListCommands()
	memdb.RegisterSetCommands()
	memdb.RegisterZSetCommands()
	memdb.RegisterInfoCommands()
}

func bindNodeFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&(config.Configures.ConfFile), "config", "c", "", "Appoint a config file: such as /etc/redis.conf")
	cmd.Flags().BoolVar(&(config.Configures.Standalone), "standalone", false, "Run in standalone mode without Raft (no clustering)")
	cmd.Flags().StringVarP(&(config.Configures.Host), "host", "H", config.DefaultHost, "Bind host ip: default is 127.0.0.1")
	cmd.Flags().IntVarP(&(config.Configures.Port), "port", "p", config.DefaultPort, "Bind a listening port: default is 6379")
	cmd.Flags().StringVarP(&(config.Configures.LogDir), "logdir", "d", config.DefaultLogDir, "Set log directory: default is /tmp")
	cmd.Flags().StringVarP(&(config.Configures.LogLevel), "loglevel", "l", config.DefaultLogLevel, "Set log level: default is info")
	cmd.Flags().BoolVar(&(config.Configures.LogSamplingEnabled), "log-sampling", config.DefaultLogSamplingEnabled, "Enable log sampling to reduce duplicate entries")
	cmd.Flags().DurationVar(&(config.Configures.LogSamplingInterval), "log-sampling-interval", config.DefaultLogSamplingInterval, "Log sampling interval window (e.g., 1s)")
	cmd.Flags().IntVar(&(config.Configures.LogSamplingInitial), "log-sampling-initial", config.DefaultLogSamplingInitial, "Number of identical logs allowed within the sampling window before dropping")
	cmd.Flags().IntVar(&(config.Configures.LogSamplingThereafter), "log-sampling-thereafter", config.DefaultLogSamplingThereafter, "Number of identical logs allowed after the initial burst within the sampling window")
	cmd.Flags().IntVarP(&(config.Configures.ShardNum), "shardnum", "s", config.DefaultShardNum, "Set shard number: default is 1024")
	cmd.Flags().StringVar(&(config.Configures.NodeID), "node-id", config.DefaultNodeID, "Unique Raft node identifier")
	cmd.Flags().StringVar(&(config.Configures.RaftDir), "raft-dir", config.DefaultRaftDir, "Directory to store Raft state")
	cmd.Flags().StringVar(&(config.Configures.RaftBind), "raft-bind", config.DefaultRaftBind, "Raft TCP bind address")
	cmd.Flags().StringVar(&(config.Configures.RaftHTTPAddr), "raft-http", config.DefaultRaftHTTPAddr, "HTTP bind address for Raft join requests")
	cmd.Flags().StringVar(&(config.Configures.JoinAddr), "raft-join", config.DefaultRaftJoinAddr, "Join address of an existing node (host:port)")
	cmd.Flags().BoolVar(&(config.Configures.RaftBootstrap), "raft-bootstrap", false, "Bootstrap cluster with this node")
	cmd.Flags().IntVar(&(config.Configures.RaftConnectionPool), "raft-pool", config.DefaultRaftConnectionPool, "Raft connection pool size")
	cmd.Flags().DurationVar(&(config.Configures.RaftTimeout), "raft-timeout", config.DefaultRaftTimeout, "Raft network timeout (e.g., 10s)")
	cmd.Flags().Uint64Var(&(config.Configures.RaftSnapshotThreshold), "raft-snap-threshold", uint64(config.DefaultRaftSnapshotThresh), "Snapshot after this many log entries")
	cmd.Flags().DurationVar(&(config.Configures.RaftSnapshotInterval), "raft-snap-interval", config.DefaultRaftSnapshotIntvl, "Snapshot interval duration (e.g., 2m)")
}

func runNode(cmd *cobra.Command) error {
	cfg, err := config.Setup(cmd)
	if err != nil {
		return err
	}
	// If a config file is specified, load and override defaults/flags accordingly.
	if cfg.ConfFile != "" {
		if err := cfg.Parse(cfg.ConfFile); err != nil {
			return err
		}
	}
	if err = logger.SetUp(cfg); err != nil {
		return err
	}
	return server.Start(cfg)
}

func Run() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
