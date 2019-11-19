#!/bin/bash
set -exu

if [ $UID -ne 0 ]
then
    # shellcheck disable=SC2068
    sudo "$0" $@
    exit $?
fi

IF_SERVER=coredhcp-server
IF_CLIENT=coredhcp-client
BRIDGE=coredhcp-bridge
BR_SERVER="br-coredhcp-se"
BR_CLIENT="br-coredhcp-cl"
NETNS_SERVER=coredhcp-server
NETNS_CLIENT=coredhcp-client

CLEANUP=1

nsexec_client() {
    # shellcheck disable=SC2068
    ip netns exec "${NETNS_CLIENT}" $@
}

nsexec_server() {
    # shellcheck disable=SC2068
    ip netns exec "${NETNS_SERVER}" $@
}

# clean-up
if [ "${CLEANUP}" -ne 0 ]
then
    nsexec_client ip link del dev "${IF_CLIENT}" || true
    nsexec_server ip link del dev "${IF_SERVER}" || true

    nsexec_client ip link del dev "${IF_CLIENT}" || true
    nsexec_server ip link del dev "${IF_SERVER}" || true

    ip link del dev "${BRIDGE}" || true

    ip netns del "${NETNS_CLIENT}" || true
    ip netns del "${NETNS_SERVER}" || true
fi

# create veth interfaces and add them to the namespace
ip netns add "${NETNS_CLIENT}"
ip netns add "${NETNS_SERVER}"

ip link add "${IF_CLIENT}" type veth peer name "${BR_CLIENT}"
ip link add "${IF_SERVER}" type veth peer name "${BR_SERVER}"

ip link set "${IF_CLIENT}" netns "${NETNS_CLIENT}"
ip link set "${IF_SERVER}" netns "${NETNS_SERVER}"

# configure networking on the veth interfaces
nsexec_server ip addr add '2001:db8:2::10/64' dev "${IF_SERVER}"
nsexec_server ip addr add '10.0.0.10/24' dev "${IF_SERVER}"
nsexec_server ip link set lo up
nsexec_server ip link set "${IF_SERVER}" up
ip link set "${BR_SERVER}" up

nsexec_client ip addr add '2001:db8:2::100/64' dev "${IF_CLIENT}"
nsexec_client ip addr add '10.0.0.100/24' dev "${IF_CLIENT}"
nsexec_client ip link set lo up
nsexec_client ip link set "${IF_CLIENT}" up
ip link set "${BR_CLIENT}" up

# configure bridging
ip link add name "${BRIDGE}" multicast on type bridge
ip link set "${BRIDGE}" up

ip link set "${BR_CLIENT}" master "${BRIDGE}"
ip link set "${BR_SERVER}" master "${BRIDGE}"

ip link set "${BR_CLIENT}" up
ip link set "${BR_SERVER}" up

ip addr add '2001:db8:2::ff/64' dev "${BRIDGE}"
ip addr add '10.0.0.254/24' brd + dev "${BRIDGE}"

# set up routes
nsexec_client ip route add default \
    via '2001:db8:2::ff'
nsexec_server ip route add default \
    via '2001:db8:2::ff'

# enable neighbour proxying
sysctl -w "net.ipv6.conf.${BRIDGE}.proxy_ndp=1"
sysctl -w "net.ipv6.conf.${BRIDGE}.forwarding=1"
sysctl -w "net.ipv6.conf.${BR_CLIENT}.proxy_ndp=1"
sysctl -w "net.ipv6.conf.${BR_CLIENT}.forwarding=1"
sysctl -w "net.ipv6.conf.${BR_SERVER}.proxy_ndp=1"
sysctl -w "net.ipv6.conf.${BR_SERVER}.forwarding=1"
ip -6 neigh add proxy '2001:db8:2::10' dev "${BRIDGE}"
ip -6 neigh add proxy '2001:db8:2::100' dev "${BRIDGE}"
ip -6 neigh add proxy 'ff01::1:2' dev "${BRIDGE}"

# show what we did
echo "## Client: ip addr list"
nsexec_client ip addr list
echo "## Server: ip addr list"
nsexec_server ip addr list
