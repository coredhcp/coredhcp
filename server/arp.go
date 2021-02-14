package server

import (
	"golang.org/x/sys/unix"
	"net"
	"os"
	"syscall"
	"unsafe"
)

// ARP Flag values
// these are not in golang.org/x/sys/unix
const (
	// completed entry (ha valid)
	ATF_COM = 0x02
	// permanent entry
	ATF_PERM = 0x04
	// publish entry
	ATF_PUBL = 0x08
	// has requested trailers
	ATF_USETRAILERS = 0x10
	// want to use a netmask (only for proxy entries)
	ATF_NETMASK = 0x20
	// don't answer this addresses
	ATF_DONTPUB = 0x40
)

// https://man7.org/linux/man-pages/man7/arp.7.html
type arpReq struct {
	ArpPa   syscall.RawSockaddrInet4
	ArpHa   syscall.RawSockaddr
	Flags   int32
	Netmask syscall.RawSockaddr
	Dev     [16]byte
}

func InjectArp(ip net.IP, mac net.HardwareAddr, flags int32, dev string) (err error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return
	}
	f := os.NewFile(uintptr(fd), "")
	defer f.Close()

	return InjectArpFd(uintptr(fd), ip, mac, flags, dev)
}

func InjectArpFd(fd uintptr, ip net.IP, mac net.HardwareAddr, flags int32, dev string) (err error) {
	arpReq := arpReq{
		ArpPa: syscall.RawSockaddrInet4{
			Family: syscall.AF_INET,
		},
		//Flags: 0x02 | 0x04, // ATF_COM | ATF_PERM;
		Flags: flags,
	}
	copy(arpReq.ArpPa.Addr[:], ip.To4())

	// uint8 to int8 conversion
	for i, b := range mac {
		arpReq.ArpHa.Data[i] = int8(b)
	}
	copy(arpReq.Dev[:], dev)

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, unix.SIOCSARP, uintptr(unsafe.Pointer(&arpReq)))
	if errno != 0 {
		return errno
	}

	return
}
