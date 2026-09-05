#define _EE
#include <string.h>
#include <ps2ips.h>
#include <netman.h>

struct ps2go_ipconfig {
	int dhcp;
	int dhcp_bound;
	unsigned int ip, netmask, gateway, dns;
};

static const char netif[] = "sm0";

int ps2go_ip_init(void) { return ps2ip_init(); }

int ps2go_ip_getconfig(struct ps2go_ipconfig *c) {
	t_ip_info info;
	if (ps2ip_getconfig((char *)netif, &info) <= 0) {
		return -1;
	}
	c->dhcp = info.dhcp_enabled;
	c->dhcp_bound = info.dhcp_enabled && info.dhcp_status == DHCP_STATE_BOUND;
	c->ip = info.ipaddr.s_addr;
	c->netmask = info.netmask.s_addr;
	c->gateway = info.gw.s_addr;
	const ip_addr_t *dns = dns_getserver(0);
	c->dns = dns ? ip_addr_get_ip4_u32(dns) : 0;
	return 0;
}

int ps2go_ip_setconfig(const struct ps2go_ipconfig *c) {
	t_ip_info info;
	if (ps2ip_getconfig((char *)netif, &info) <= 0) {
		return -1;
	}
	info.dhcp_enabled = c->dhcp;
	info.ipaddr.s_addr = c->ip;
	info.netmask.s_addr = c->netmask;
	info.gw.s_addr = c->gateway;
	return ps2ip_setconfig(&info) <= 0 ? -2 : 0;
}

void ps2go_dns_setserver(unsigned int dns) {
	ip_addr_t a;
	ip_addr_set_ip4_u32(&a, dns);
	dns_setserver(0, &a);
}
