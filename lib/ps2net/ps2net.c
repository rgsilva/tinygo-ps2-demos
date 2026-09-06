// Sockets for the netdev driver, straight on the ps2ips RPC client
// (ps2ipc_*: the EE side of ps2ips.irx). The libc glue on top of it is not
// used: its close() does not release the IOP socket, and it loses errno.
// Failures return -errno as lwip reports it (SO_ERROR), -EIO when it does
// not say. The sockets stay blocking; the Go side only calls recv/send/
// accept after ps2go_ready says they will not block.
#define _EE
#include <errno.h>
#include <string.h>
#include <tamtypes.h>
#include <kernel.h>
#include <sifrpc.h>
#include <ps2ip_rpc.h>
#include <ps2ips.h>
#include <sys/socket.h>

extern int ps2ipc_socket(int domain, int type, int protocol);
extern int ps2ipc_bind(int s, const struct sockaddr *name, int namelen);
extern int ps2ipc_connect(int s, const struct sockaddr *name, int namelen);
extern int ps2ipc_listen(int s, int backlog);
extern int ps2ipc_accept(int s, struct sockaddr *addr, int *addrlen);
extern int ps2ipc_send(int s, const void *dataptr, int size, unsigned int flags);
extern int ps2ipc_recv(int s, void *mem, int len, unsigned int flags);
extern int ps2ipc_disconnect(int s);
extern int ps2ipc_select(int maxfdp1, struct fd_set *readset, struct fd_set *writeset, struct fd_set *exceptset, struct timeval *timeout);
extern int ps2ipc_ioctl(int s, long cmd, void *argp);
extern int ps2ipc_getsockopt(int s, int level, int optname, void *optval, socklen_t *optlen);
extern int ps2ipc_setsockopt(int s, int level, int optname, const void *optval, socklen_t optlen);

// Per send/recv RPC the IOP stages the data in a 1024+32 byte buffer, at
// offset 64 when the EE address is aligned: 960 bytes fit, 1024 overrun it.
#define RPC_CHUNK 960

// The socket's pending error (SO_ERROR), 0 when lwip has none recorded:
// a non-blocking call that would block leaves none.
static int pending(int s) {
	int err = 0;
	socklen_t len = sizeof(err);
	if (ps2ipc_getsockopt(s, SOL_SOCKET, SO_ERROR, &err, &len) < 0) {
		return EBADF; // the RPC itself failed
	}
	return err;
}

static int fail(int s) {
	int err = pending(s);
	return err > 0 ? -err : -EIO;
}


int ps2go_socket(int domain, int type, int proto) {
	int d = domain == 2 ? AF_INET : -1;
	int t = type == 1 ? SOCK_STREAM : type == 2 ? SOCK_DGRAM : -1;
	int p = proto == 6 ? IPPROTO_TCP : proto == 17 ? IPPROTO_UDP : proto == 0 ? 0 : -1;
	if (d < 0 || t < 0 || p < 0) {
		return -EPROTONOSUPPORT;
	}
	int s = ps2ipc_socket(d, t, p);
	return s < 0 ? -ENFILE : s;
}

static void addr(struct sockaddr_in *a, unsigned int ip, unsigned short port) {
	memset(a, 0, sizeof(*a));
	a->sin_len = sizeof(*a);
	a->sin_family = AF_INET;
	a->sin_port = htons(port);
	a->sin_addr.s_addr = ip;
}

int ps2go_bind(int s, unsigned int ip, unsigned short port) {
	struct sockaddr_in a;
	addr(&a, ip, port);
	return ps2ipc_bind(s, (struct sockaddr *)&a, sizeof(a)) < 0 ? fail(s) : 0;
}

int ps2go_connect(int s, unsigned int ip, unsigned short port) {
	struct sockaddr_in a;
	addr(&a, ip, port);
	return ps2ipc_connect(s, (struct sockaddr *)&a, sizeof(a)) < 0 ? fail(s) : 0;
}

int ps2go_listen(int s, int backlog) {
	return ps2ipc_listen(s, backlog) < 0 ? fail(s) : 0;
}

// Accepts a connection; the peer's address goes to *ip and *port.
int ps2go_accept(int s, unsigned int *ip, unsigned short *port) {
	struct sockaddr_in a;
	int len = sizeof(a);
	int c = ps2ipc_accept(s, (struct sockaddr *)&a, &len);
	if (c < 0) {
		return fail(s);
	}
	*ip = a.sin_addr.s_addr;
	*port = ntohs(a.sin_port);
	return c;
}

// Sends as much as the socket takes right now, in RPC-sized pieces.
int ps2go_send(int s, const void *buf, int len, int flags) {
	int done = 0;
	while (done < len) {
		int n = len - done > RPC_CHUNK ? RPC_CHUNK : len - done;
		int r = ps2ipc_send(s, (const char *)buf + done, n, flags);
		if (r < 0) {
			return done > 0 ? done : fail(s);
		}
		done += r;
		if (r < n) {
			break;
		}
	}
	return done;
}

// Receives are our own RPC call to ps2ips, not the library's ps2ipc_recv:
// the IOP finishes a receive by DMAing a 144-byte completion record
// (rests_pkt: the unaligned head and tail bytes) to a buffer the caller
// names, and the library's buffer is 128 bytes, so the last 16 bytes of a
// tail longer than 48 bytes were overwritten by the RPC reply. The data
// goes to a 64-byte aligned buffer: the IOP then DMAs the whole middle
// (behind the data cache, read through the uncached mirror) and the
// completion handler copies the tail with the CPU.
static SifRpcClientData_t recv_rpc __attribute__((aligned(64)));
static union {
	s_recv_pkt s;
	r_recv_pkt r;
} recv_pkt __attribute__((aligned(64)));
static rests_pkt recv_rests __attribute__((aligned(64)));
static char recv_buf[RPC_CHUNK] __attribute__((aligned(64)));
#define UNCACHED(p) ((void *)((unsigned int)(p) | 0x20000000))

// Runs when the reply arrives: the head and tail bytes into place.
static void recv_done(void *data) {
	rests_pkt *rests = UNCACHED(data);
	for (int i = 0; i < rests->ssize; i++) {
		rests->sbuf[i] = rests->sbuffer[i];
	}
	for (int i = 0; i < rests->esize; i++) {
		rests->ebuf[i] = rests->ebuffer[i];
	}
}

int ps2go_recv(int s, void *buf, int len, int flags) {
	if (recv_rpc.server == NULL) {
		while (sceSifBindRpc(&recv_rpc, PS2IP_IRX, 0) < 0 || recv_rpc.server == NULL) {
			nopdelay();
		}
	}
	int n = len > RPC_CHUNK ? RPC_CHUNK : len;
	recv_pkt.s.socket = s;
	recv_pkt.s.length = n;
	recv_pkt.s.flags = flags;
	recv_pkt.s.ee_addr = recv_buf;
	recv_pkt.s.intr_data = &recv_rests;
	SyncDCache(recv_buf, recv_buf + sizeof(recv_buf));
	if (sceSifCallRpc(&recv_rpc, PS2IPS_ID_RECV, 0, &recv_pkt.s, sizeof(recv_pkt.s), &recv_pkt.r, sizeof(recv_pkt.r), recv_done, &recv_rests) < 0) {
		return -EIO;
	}
	int r = ((r_recv_pkt *)UNCACHED(&recv_pkt))->ret;
	if (r < 0) {
		// A failure with no error recorded is the peer being gone
		// (reset, or closed and already reported): end of stream.
		int err = pending(s);
		return err > 0 ? -err : 0;
	}
	if (r <= 64) {
		memcpy(buf, recv_buf, r);
	} else {
		int middle = r & ~63;
		memcpy(buf, UNCACHED(recv_buf), middle);
		memcpy((char *)buf + middle, recv_buf + middle, r - middle);
	}
	return r;
}

// Is the socket readable (want 0) or writable (want 1) right now? 1, 0,
// or -errno. Used before every recv/send/accept: a recv that finds no
// data leaves the RPC's completion data stale, and the next completion
// then scribbles over the old buffer.
int ps2go_ready(int s, int want) {
	struct fd_set set;
	struct timeval tv = {0, 0};
	FD_ZERO(&set);
	FD_SET(s, &set);
	int n = ps2ipc_select(s + 1, want == 0 ? &set : NULL, want == 1 ? &set : NULL, NULL, &tv);
	return n < 0 ? fail(s) : (n > 0 && FD_ISSET(s, &set));
}

int ps2go_eagain(void) { return EAGAIN; }

int ps2go_close(int s) {
	return ps2ipc_disconnect(s) < 0 ? -EIO : 0;
}

// Socket options by netdev's numbering: level 1 = SOL_SOCKET (opt 9 =
// SO_KEEPALIVE), level 6 = TCP (opt 1 = TCP_NODELAY).
int ps2go_setsockopt(int s, int level, int opt, int value) {
	int l, o;
	if (level == 1 && opt == 9) {
		l = SOL_SOCKET; o = SO_KEEPALIVE;
	} else if (level == 6 && opt == 1) {
		l = IPPROTO_TCP; o = TCP_NODELAY;
	} else {
		return -ENOPROTOOPT;
	}
	return ps2ipc_setsockopt(s, l, o, &value, sizeof(value)) < 0 ? fail(s) : 0;
}

const char *ps2go_strerror(int e) { return strerror(e); }
