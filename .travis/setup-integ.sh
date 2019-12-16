#!/bin/bash
set -ex

if [[ $UID -ne 0 ]] # TODO: check for permissions instead (we can have CAP_NET_ADMIN without root)
then
    sudo "$0" "$@"
    exit $?
fi

# Topology: with one or 3 netns
#
# * 3-netns, for relay operations
#  --------------------------------
# | server (cdhcp_srv  <---------) | Upper netns
#  -----------------------------|--
#                               | (veth pair)
#  -----------------------------|---
# | relay upper (cdhcp_relay_u <-) |
# |                                | Relay netns
# | relay lower (cdhcp_relay_d <-) |
#  -----------------------------|--
#                               | (veth pair)
# ------------------------------|--
# |  client (cdhcp_cli <---------) | Lower netns
# ---------------------------------
#
# For 2-netns operation, remove the entire middle layer:
#
#  --------------------------------
# | server (cdhcp_srv  <---------) | Upper netns
#  -----------------------------|--
#                               | (veth pair)
# ------------------------------|--
# |  client (cdhcp_cli <---------) | Lower netns
# ---------------------------------
#


# Interface names are limited to 15 chars (IFNAMSIZ=16)
if_server=cdhcp_srv
if_relay_up=cdhcp_relay_u
if_relay_down=cdhcp_relay_d
if_client=cdhcp_cli

netns_server=coredhcp-upper
netns_relay=coredhcp-middle
netns_client=coredhcp-lower

netns_direct_server=coredhcp-direct-upper
netns_direct_client=coredhcp-direct-lower

ula_prefix=${ULA_PREFIX:-fd4f:6b37:542c:b643}

all_ns=("$netns_server" "$netns_relay" "$netns_client" "$netns_direct_server" "$netns_direct_client")

# Clean existing namespaces
for netns in "${all_ns[@]}"; do
    ip netns delete "$netns" || true
done
[[ $1 == teardown ]] && exit

# create namespaces
for netns in "${all_ns[@]}"; do
    ip netns add "$netns"
done

# Create the links in one of the relevant netns, to ensure we don't pollute the main netns
ip -n "$netns_client" link add "$if_client" type veth peer name "$if_relay_down"
ip -n "$netns_client" link set "$if_relay_down" netns "$netns_relay"
ip -n "$netns_server" link add "$if_server" type veth peer name "$if_relay_up"
ip -n "$netns_server" link set "$if_relay_up" netns "$netns_relay"

# configure networking on the veth interfaces
ip -n "$netns_server" addr add "${ula_prefix}:a::1/80" dev "$if_server"
ip -n "$netns_server" addr add "10.0.1.1/24" dev "$if_server"
ip -n "$netns_server" link set "$if_server" up

ip -n "$netns_client" addr add "${ula_prefix}:b::1/80" dev "$if_client"
ip -n "$netns_client" addr add "10.0.2.1/24" dev "$if_client"
ip -n "$netns_client" link set "$if_client" up

ip -n "$netns_relay" addr add "${ula_prefix}:b::2/80" dev "$if_relay_down"
ip -n "$netns_relay" addr add "${ula_prefix}:a::2/80" dev "$if_relay_up"
ip -n "$netns_relay" addr add "10.0.2.2/24" dev "$if_relay_down"
ip -n "$netns_relay" addr add "10.0.1.2/24" dev "$if_relay_up"
ip -n "$netns_relay" link set "$if_relay_down" up
ip -n "$netns_relay" link set "$if_relay_up" up

# Now setup the direct-attach ns (with the same addresses as in the relay scenario)
ip -n "$netns_direct_client" link add "$if_client" type veth peer name "$if_server"
ip -n "$netns_direct_client" link set "$if_server" netns "$netns_direct_server"

# Use the same addresses as the direct-attached version; with a larger subnet so they can link
ip -n "$netns_direct_server" addr add "${ula_prefix}:a::1/64" dev "$if_server"
ip -n "$netns_direct_server" addr add "10.0.1.1/16" dev "$if_server"
ip -n "$netns_direct_server" link set "$if_server" up

ip -n "$netns_direct_client" addr add "${ula_prefix}:b::1/64" dev "$if_client"
ip -n "$netns_direct_client" addr add "10.0.2.1/16" dev "$if_client"
ip -n "$netns_direct_client" link set "$if_client" up

# show what we did
set +x
for netns in "${all_ns[@]}"; do
    echo "# Addresses in $netns:"
    ip -n "$netns" address list
done
