#define _EE
#include <netman.h>

int ps2go_netman_init(void) { return NetManInit(); }
int ps2go_netman_set_link_mode(int mode) { return NetManSetLinkMode(mode); }
int ps2go_netman_link_up(void) { return NetManGetGlobalNetIFLinkState() == NETMAN_NETIF_ETH_LINK_STATE_UP; }
