package memdb

import (
	"strings"

	"github.com/hsn0918/tinyredis/pkg/RESP"
	"github.com/hsn0918/tinyredis/pkg/logger"
)

func RegisterInfoCommands() {
	RegisterCommand("client", client)
	RegisterCommand("config", infoConfig)
	RegisterCommand("scan", scan)
	RegisterCommand("info", info)
	RegisterCommand("quit", quit)

}

// client
func client(m *MemDb, cmd [][]byte) RESP.RedisData {
	//fmt.Println("client setname 127.0.0.1@6379 done")
	return RESP.MakeBulkData([]byte("OK"))
}

// config
func infoConfig(m *MemDb, cmd [][]byte) RESP.RedisData {
	//todo:config
	//fmt.Println("config")
	return RESP.MakeBulkData([]byte("OK"))
}

// scan
func scan(m *MemDb, cmd [][]byte) RESP.RedisData {
	//todo:scan
	//fmt.Println("scan")
	return RESP.MakeNullBulkData()
}

// quit
func quit(m *MemDb, cmd [][]byte) RESP.RedisData {
	//todo:quit

	return RESP.MakeBulkData([]byte("OK"))
}

// info
func info(m *MemDb, cmd [][]byte) RESP.RedisData {
	cmdName := string(cmd[0])
	if strings.ToLower(cmdName) != "info" {
		logger.Error("info Function: cmdName is not info")
		return RESP.MakeErrorData("server error")
	}
	if len(cmd) > 2 {
		return RESP.MakeErrorData("error: command args number is invalid")
	}
	var infoStr = "# Server\nredis_version:6.2.6\nredis_git_sha1:00000000\nredis_git_dirty:0\nredis_build_id:b61f37314a089f19\nredis_mode:standalone\nos:Linux 5.4.0-163-generic x86_64\narch_bits:64\nmultiplexing_api:epoll\natomicvar_api:atomic-builtin\ngcc_version:10.2.1\nprocess_id:1\nprocess_supervised:no\nrun_id:5821cfc903a866e3bfed875c7fa62433739af927\ntcp_port:6379\nserver_time_usec:1711703004547834\nuptime_in_seconds:323959\nuptime_in_days:3\nhz:10\nconfigured_hz:10\nlru_clock:426972\nexecutable:/data/redis-server\nconfig_file:/etc/redis/redis.conf\nio_threads_active:0\n\n# Clients\nconnected_clients:2\ncluster_connections:0\nmaxclients:10000\nclient_recent_max_input_buffer:56\nclient_recent_max_output_buffer:0\nblocked_clients:0\ntracking_clients:0\nclients_in_timeout_table:0\n\n# Memory\nused_memory:904768\nused_memory_human:883.56K\nused_memory_rss:7176192\nused_memory_rss_human:6.84M\nused_memory_peak:964896\nused_memory_peak_human:942.28K\nused_memory_peak_perc:93.77%\nused_memory_overhead:851560\nused_memory_startup:810144\nused_memory_dataset:53208\nused_memory_dataset_perc:56.23%\nallocator_allocated:936424\nallocator_active:1261568\nallocator_resident:4075520\ntotal_system_memory:16773562368\ntotal_system_memory_human:15.62G\nused_memory_lua:37888\nused_memory_lua_human:37.00K\nused_memory_scripts:0\nused_memory_scripts_human:0B\nnumber_of_cached_scripts:0\nmaxmemory:0\nmaxmemory_human:0B\nmaxmemory_policy:noeviction\nallocator_frag_ratio:1.35\nallocator_frag_bytes:325144\nallocator_rss_ratio:3.23\nallocator_rss_bytes:2813952\nrss_overhead_ratio:1.76\nrss_overhead_bytes:3100672\nmem_fragmentation_ratio:8.32\nmem_fragmentation_bytes:6314152\nmem_not_counted_for_evict:4\nmem_replication_backlog:0\nmem_clients_slaves:0\nmem_clients_normal:41032\nmem_aof_buffer:8\nmem_allocator:jemalloc-5.1.0\nactive_defrag_running:0\nlazyfree_pending_objects:0\nlazyfreed_objects:0\n\n# Persistence\nloading:0\ncurrent_cow_size:0\ncurrent_cow_size_age:0\ncurrent_fork_perc:0.00\ncurrent_save_keys_processed:0\ncurrent_save_keys_total:0\nrdb_changes_since_last_save:0\nrdb_bgsave_in_progress:0\nrdb_last_save_time:1711382646\nrdb_last_bgsave_status:ok\nrdb_last_bgsave_time_sec:0\nrdb_current_bgsave_time_sec:-1\nrdb_last_cow_size:315392\naof_enabled:1\naof_rewrite_in_progress:0\naof_rewrite_scheduled:0\naof_last_rewrite_time_sec:-1\naof_current_rewrite_time_sec:-1\naof_last_bgrewrite_status:ok\naof_last_write_status:ok\naof_last_cow_size:0\nmodule_fork_in_progress:0\nmodule_fork_last_cow_size:0\naof_current_size:665\naof_base_size:665\naof_pending_rewrite:0\naof_buffer_length:0\naof_rewrite_buffer_length:0\naof_pending_bio_fsync:0\naof_delayed_fsync:0\n\n# Stats\ntotal_connections_received:320\ntotal_commands_processed:1837\ninstantaneous_ops_per_sec:0\ntotal_net_input_bytes:32814\ntotal_net_output_bytes:907585\ninstantaneous_input_kbps:0.00\ninstantaneous_output_kbps:0.00\nrejected_connections:0\nsync_full:0\nsync_partial_ok:0\nsync_partial_err:0\nexpired_keys:0\nexpired_stale_perc:0.00\nexpired_time_cap_reached_count:0\nexpire_cycle_cpu_milliseconds:9807\nevicted_keys:0\nkeyspace_hits:20\nkeyspace_misses:0\npubsub_channels:0\npubsub_patterns:0\nlatest_fork_usec:716\ntotal_forks:1\nmigrate_cached_sockets:0\nslave_expires_tracked_keys:0\nactive_defrag_hits:0\nactive_defrag_misses:0\nactive_defrag_key_hits:0\nactive_defrag_key_misses:0\ntracking_total_keys:0\ntracking_total_items:0\ntracking_total_prefixes:0\nunexpected_error_replies:0\ntotal_error_replies:1757\ndump_payload_sanitizations:0\ntotal_reads_processed:2383\ntotal_writes_processed:2069\nio_threaded_reads_processed:0\nio_threaded_writes_processed:0\n\n# Replication\nrole:master\nconnected_slaves:0\nmaster_failover_state:no-failover\nmaster_replid:691ddf41902e6b7f474c89322ee984e920efc8f3\nmaster_replid2:0000000000000000000000000000000000000000\nmaster_repl_offset:0\nsecond_repl_offset:-1\nrepl_backlog_active:0\nrepl_backlog_size:1048576\nrepl_backlog_first_byte_offset:0\nrepl_backlog_histlen:0\n\n# CPU\nused_cpu_sys:341.905416\nused_cpu_user:372.496196\nused_cpu_sys_children:0.010041\nused_cpu_user_children:0.002399\nused_cpu_sys_main_thread:341.816908\nused_cpu_user_main_thread:372.468378\n\n# Modules\n\n# Errorstats\nerrorstat_ERR:count=8\nerrorstat_NOAUTH:count=233\nerrorstat_WRONGPASS:count=1516\n\n# Cluster\ncluster_enabled:0\n\n# Keyspace\ndb0:keys=2,expires=0,avg_ttl=0\ndb2:keys=5,expires=0,avg_ttl=0:/Users/ming/Desktop/godis/redis.conf\n# Clients\nconnected_clients:1\n# Cluster\ncluster_enabled:0\n# Keyspace\ndb0:keys=5,expires=0,avg_ttl=0\n\n# Server\ngodis_version:1.2.8\ngodis_mode:standalone\nos:darwin arm64\narch_bits:64\ngo_version:go1.21.6\nprocess_id:69684\nrun_id:lPepFMBbQtEYt3MD5x712p4rCQHClYU2G1xM6k5t\ntcp_port:6399\nuptime_in_seconds:5\nuptime_in_days:0\nconfig_file:/Users/redis.conf\n# Clients\nconnected_clients:1\n# Cluster\ncluster_enabled:0\n# Keyspace\ndb0:keys=5,expires=0,avg_ttl=0\n"
	if len(cmd) == 1 {
		return RESP.MakeBulkData([]byte(infoStr))
	}
	return RESP.MakeBulkData([]byte(infoStr))
}
