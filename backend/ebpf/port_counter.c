//go:build ignore

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define MAX_PORTS 65536

struct port_stats {
    __u64 rx_bytes;
    __u64 tx_bytes;
    __u64 rx_packets;
    __u64 tx_packets;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, MAX_PORTS);
    __type(key, __u32);
    __type(value, struct port_stats);
} port_counters SEC(".maps");

static __always_inline int parse_and_count(struct __sk_buff *skb, int is_ingress) {
    void *data_end = (void *)(long)skb->data_end;
    void *data = (void *)(long)skb->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return TC_ACT_OK;

    __u16 proto = eth->h_proto;
    __u8 ip_proto = 0;
    __u16 src_port = 0, dst_port = 0;

    if (proto == bpf_htons(ETH_P_IP)) {
        struct iphdr *ip = (void *)(eth + 1);
        if ((void *)(ip + 1) > data_end)
            return TC_ACT_OK;

        ip_proto = ip->protocol;
        __u32 ihl = ip->ihl * 4;

        if (ip_proto == IPPROTO_TCP) {
            struct tcphdr *tcp = (void *)ip + ihl;
            if ((void *)(tcp + 1) > data_end)
                return TC_ACT_OK;
            src_port = bpf_ntohs(tcp->source);
            dst_port = bpf_ntohs(tcp->dest);
        } else if (ip_proto == IPPROTO_UDP) {
            struct udphdr *udp = (void *)ip + ihl;
            if ((void *)(udp + 1) > data_end)
                return TC_ACT_OK;
            src_port = bpf_ntohs(udp->source);
            dst_port = bpf_ntohs(udp->dest);
        }
    } else if (proto == bpf_htons(ETH_P_IPV6)) {
        struct ipv6hdr *ip6 = (void *)(eth + 1);
        if ((void *)(ip6 + 1) > data_end)
            return TC_ACT_OK;

        ip_proto = ip6->nexthdr;

        if (ip_proto == IPPROTO_TCP) {
            struct tcphdr *tcp = (void *)(ip6 + 1);
            if ((void *)(tcp + 1) > data_end)
                return TC_ACT_OK;
            src_port = bpf_ntohs(tcp->source);
            dst_port = bpf_ntohs(tcp->dest);
        } else if (ip_proto == IPPROTO_UDP) {
            struct udphdr *udp = (void *)(ip6 + 1);
            if ((void *)(udp + 1) > data_end)
                return TC_ACT_OK;
            src_port = bpf_ntohs(udp->source);
            dst_port = bpf_ntohs(udp->dest);
        }
    }

    if (src_port || dst_port) {
        __u32 bytes = skb->len;
        __u32 port = is_ingress ? dst_port : src_port;
        struct port_stats *stats = bpf_map_lookup_elem(&port_counters, &port);
        if (stats) {
            if (is_ingress) {
                __sync_fetch_and_add(&stats->rx_bytes, bytes);
                __sync_fetch_and_add(&stats->rx_packets, 1);
            } else {
                __sync_fetch_and_add(&stats->tx_bytes, bytes);
                __sync_fetch_and_add(&stats->tx_packets, 1);
            }
        }
    }

    return TC_ACT_OK;
}

SEC("tc/ingress")
int tc_ingress(struct __sk_buff *skb) {
    return parse_and_count(skb, 1);
}

SEC("tc/egress")
int tc_egress(struct __sk_buff *skb) {
    return parse_and_count(skb, 0);
}

char _license[] SEC("license") = "GPL";
