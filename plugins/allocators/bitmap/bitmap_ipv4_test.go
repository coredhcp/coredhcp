// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package bitmap

import (
	"net"
	"testing"
)

func getv4Allocator() *IPv4Allocator {
	alloc, err := NewIPv4Allocator(net.IPv4(192, 0, 2, 0), net.IPv4(192, 0, 2, 255))
	if err != nil {
		panic(err)
	}

	return alloc
}
func Test4Alloc(t *testing.T) {
	alloc := getv4Allocator()

	net1, err := alloc.Allocate(net.IPNet{})
	if err != nil {
		t.Fatal(err)
	}

	net2, err := alloc.Allocate(net.IPNet{})
	if err != nil {
		t.Fatal(err)
	}

	if net1.IP.Equal(net2.IP) {
		t.Fatal("That address was already allocated")
	}

	err = alloc.Free(net1)
	if err != nil {
		t.Fatal(err)
	}

	err = alloc.Free(net1)
	if err == nil {
		t.Fatal("Expected DoubleFree error")
	}
}

func Test4OutOfPool(t *testing.T) {
	alloc := getv4Allocator()

	hint := net.IPv4(198, 51, 100, 5)
	res, err := alloc.Allocate(net.IPNet{IP: hint, Mask: net.CIDRMask(32, 32)})
	if err != nil {
		t.Fatalf("Failed to allocate with invalid hint: %v", err)
	}
	_, prefix, _ := net.ParseCIDR("192.0.2.0/24")
	if !prefix.Contains(res.IP) {
		t.Fatal("Obtained prefix outside of range: ", res)
	}
	if prefLen, totalLen := res.Mask.Size(); prefLen != 32 || totalLen != 32 {
		t.Fatalf("Prefixes have wrong size %d/%d", prefLen, totalLen)
	}
}
