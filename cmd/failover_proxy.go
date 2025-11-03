package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/hsn0918/tinyredis/pkg/proxy"
)

var (
	proxyListenAddr    string
	proxyTargets       []string
	proxyDialTimeout   time.Duration
	proxyRetryInterval time.Duration
)

var failoverProxyCmd = &cobra.Command{
	Use:   "failover-proxy",
	Short: "Start a lightweight TCP proxy that always forwards to the current Raft leader",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := make([]string, 0, len(proxyTargets))
		for _, t := range proxyTargets {
			targets = append(targets, strings.TrimSpace(t))
		}
		if len(targets) == 0 {
			return errors.New("at least one --nodes address must be provided")
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		p := &proxy.Proxy{
			ListenAddr:    proxyListenAddr,
			Targets:       targets,
			DialTimeout:   proxyDialTimeout,
			RetryInterval: proxyRetryInterval,
		}
		return p.Run(ctx)
	},
}

func init() {
	failoverProxyCmd.Flags().StringVar(&proxyListenAddr, "listen", "127.0.0.1:7390", "listen address for the proxy (host:port)")
	failoverProxyCmd.Flags().StringSliceVar(&proxyTargets, "nodes", nil, "RESP addresses of cluster nodes to probe (comma-separated or repeated)")
	failoverProxyCmd.Flags().DurationVar(&proxyDialTimeout, "dial-timeout", 2*time.Second, "timeout for dialing cluster nodes")
	failoverProxyCmd.Flags().DurationVar(&proxyRetryInterval, "retry-interval", time.Second, "wait time before retrying leader discovery")
	rootCmd.AddCommand(failoverProxyCmd)
}
