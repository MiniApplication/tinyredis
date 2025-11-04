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
	proxyNodeAddrs     []string
	proxyDialTimeout   time.Duration
	proxyRetryInterval time.Duration
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Start a RESP proxy that forwards commands to the current Raft leader",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := sanitizeNodeList(proxyNodeAddrs)
		if len(targets) == 0 {
			return errors.New("at least one --nodes address must be provided (comma separated or repeated)")
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
	proxyCmd.Flags().StringVar(&proxyListenAddr, "listen", "127.0.0.1:7390", "proxy listen address (host:port)")
	proxyCmd.Flags().StringSliceVar(&proxyNodeAddrs, "nodes", nil, "RESP endpoints of cluster nodes (comma separated or repeated)")
	proxyCmd.Flags().DurationVar(&proxyDialTimeout, "dial-timeout", 2*time.Second, "timeout when dialing cluster nodes")
	proxyCmd.Flags().DurationVar(&proxyRetryInterval, "retry-interval", time.Second, "wait before retrying leader discovery")
	rootCmd.AddCommand(proxyCmd)
}

func sanitizeNodeList(addrs []string) []string {
	if len(addrs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(addrs))
	var out []string
	for _, raw := range addrs {
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}
