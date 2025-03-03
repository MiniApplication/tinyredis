package cmd

import (
	"fmt"
	"os"

	"github.com/hsn/tiny-redis/pkg/config"
	"github.com/hsn/tiny-redis/pkg/logger"
	"github.com/hsn/tiny-redis/pkg/memdb"
	"github.com/hsn/tiny-redis/pkg/server"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tiny-redis",
	Short: "A tiny Redis server",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Setup(cmd)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		err = logger.SetUp(cfg)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = server.Start(cfg)
		if err != nil {
			os.Exit(1)
		}
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
			rootCmd.GenBashCompletion(os.Stdout)
		} else if len(args) > 0 && args[0] == "zsh" {
			rootCmd.GenZshCompletion(os.Stdout)
		} else if len(args) > 0 && args[0] == "fish" {
			rootCmd.GenFishCompletion(os.Stdout, true)
		} else {
			fmt.Println("Please specify a shell: bash, zsh, or fish")
		}
	},
}

func init() {
	config.Configures = config.NewDefaultConfig()
	rootCmd.Flags().StringVarP(&(config.Configures.ConfFile), "config", "c", "", "Appoint a config file: such as /etc/redis.conf")
	rootCmd.Flags().StringVarP(&(config.Configures.Host), "host", "H", config.DefaultHost, "Bind host ip: default is 127.0.0.1")
	rootCmd.Flags().IntVarP(&(config.Configures.Port), "port", "p", config.DefaultPort, "Bind a listening port: default is 6379")
	rootCmd.Flags().StringVarP(&(config.Configures.LogDir), "logdir", "d", config.DefaultLogDir, "Set log directory: default is /tmp")
	rootCmd.Flags().StringVarP(&(config.Configures.LogLevel), "loglevel", "l", config.DefaultLogLevel, "Set log level: default is info")
	rootCmd.Flags().IntVarP(&(config.Configures.ShardNum), "shardnum", "s", config.DefaultShardNum, "Set shard number: default is 1024")
	rootCmd.AddCommand(completionCmd)
	memdb.RegisterKeyCommand()
	memdb.RegisterStringCommands()
	memdb.RegisterHashCommands()
	memdb.RegisterListCommands()
	memdb.RegisterSetCommands()
	memdb.RegisterZSetCommands()
	memdb.RegisterInfoCommands()
}
func Run() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
