// SPDX-License-Identifier: GPL-2.0
//
// PortSleuth eBPF XDP program for per-port byte/packet counting.
// Attached to ingress on a network interface; egress is collected via TC.
//
// The map key is a (proto, port) pair; the value is byte/packet counters.
// Userspace reads & resets via cilium/ebpf.

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define DIR_RX 0
#define DIR_TX 1

struct port_key {
    __u16 port;
    __u8  proto;   // IPPROTO_TCP or IPPROTO_UDP
    __u8  dir;     // DIR_RX or DIR_TX
};

struct port_val {
    __u64 bytes;
    __u64 packets;
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 65536);
    __type(key, struct port_key);
    __type(value, struct port_val);
} port_counters SEC(".maps");

static __always_inline int handle_ipv4(void *data, void *data_end, __u8 dir) {
    struct iphdr *ip = data;
    if ((void *)(ip + 1) > data_end) return XDP_PASS;
    if (ip->ihl < 5) return XDP_PASS;

    void *l4 = (void *)ip + (ip->ihl * 4);
    __u16 sport = 0, dport = 0;
    __u8 proto = ip->protocol;

    if (proto == IPPROTO_TCP) {
        struct tcphdr *t = l4;
        if ((void *)(t + 1) > data_end) return XDP_PASS;
        sport = bpf_ntohs(t->source);
        dport = bpf_ntohs(t->dest);
    } else if (proto == IPPROTO_UDP) {
        struct udphdr *u = l4;
        if ((void *)(u + 1) > data_end) return XDP_PASS;
        sport = bpf_ntohs(u->source);
        dport = bpf_ntohs(u->dest);
    } else {
        return XDP_PASS;
    }

    // For RX: dport is local port (incoming traffic to that port)
    // For TX: sport is local port (outgoing from that port)
    __u16 local_port = (dir == DIR_RX) ? dport : sport;

    struct port_key key = {
        .port = local_port,
        .proto = proto,
        .dir = dir,
    };

    __u64 pkt_len = (__u64)((char *)data_end - (char *)data);

    struct port_val *val = bpf_map_lookup_elem(&port_counters, &key);
    if (val) {
        __sync_fetch_and_add(&val->bytes, pkt_len);
        __sync_fetch_and_add(&val->packets, 1);
    } else {
        struct port_val newv = { .bytes = pkt_len, .packets = 1 };
        bpf_map_update_elem(&port_counters, &key, &newv, BPF_ANY);
    }

    return XDP_PASS;
}

SEC("xdp")
int xdp_count_ingress(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return XDP_PASS;

    if (eth->h_proto == bpf_htons(ETH_P_IP)) {
        return handle_ipv4(eth + 1, data_end, DIR_RX);
    }
    return XDP_PASS;
}

// Egress is attached as a TC classifier; both directions share the same map.
SEC("tc")
int tc_count_egress(struct __sk_buff *skb) {
    void *data_end = (void *)(long)skb->data_end;
    void *data = (void *)(long)skb->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return 0; // TC_ACT_OK

    if (eth->h_proto == bpf_htons(ETH_P_IP)) {
        handle_ipv4(eth + 1, data_end, DIR_TX);
    }
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
