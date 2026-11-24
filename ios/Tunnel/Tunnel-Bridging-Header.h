// Finds the utun file descriptor that NEPacketTunnelProvider opened for us,
// so the Go core can read and write packets on it directly (the approach
// WireGuard for iOS uses; packetFlow.value(forKey:) stopped working in iOS 15).
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/ioctl.h>
#include <string.h>

#ifndef AF_SYSTEM
#define AF_SYSTEM 32
#endif
#define DIKEC_CTLIOCGINFO 0xc0644e03UL

struct dikec_ctl_info {
    u_int32_t ctl_id;
    char ctl_name[96];
};

struct dikec_sockaddr_ctl {
    u_char sc_len;
    u_char sc_family;
    u_int16_t ss_sysaddr;
    u_int32_t sc_id;
    u_int32_t sc_unit;
    u_int32_t sc_reserved[5];
};

static inline int dikec_find_utun_fd(void) {
    struct dikec_ctl_info info;
    memset(&info, 0, sizeof(info));
    strncpy(info.ctl_name, "com.apple.net.utun_control", sizeof(info.ctl_name) - 1);
    for (int fd = 0; fd <= 1024; fd++) {
        struct dikec_sockaddr_ctl addr;
        socklen_t len = sizeof(addr);
        memset(&addr, 0, sizeof(addr));
        if (getpeername(fd, (struct sockaddr *)&addr, &len) != 0 || addr.sc_family != AF_SYSTEM) continue;
        if (info.ctl_id == 0 && ioctl(fd, DIKEC_CTLIOCGINFO, &info) != 0) continue;
        if (addr.sc_id == info.ctl_id) return fd;
    }
    return -1;
}
