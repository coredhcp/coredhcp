// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package bitmap

import (
	"net"
	"testing"
)

func getAllocator() *Allocator {
	_, prefix, err := net.ParseCIDR("2001:db8::/56")
	if err != nil {
		panic(err)
	}
	alloc, err := NewBitmapAllocator(*prefix, 64)
	if err != nil {
		panic(err)
	}

	return alloc
}
func TestAlloc(t *testing.T) {
	alloc := getAllocator()

	net, err := alloc.Allocate(net.IPNet{})
	if err != nil {
		t.Fatal(err)
	}

	err = alloc.Free(net)
	if err != nil {
		t.Fatal(err)
	}

	err = alloc.Free(net)
	if err == nil {
		t.Fatal("Expected DoubleFree error")
	}
}

func TestExhaust(t *testing.T) {
	_, prefix, _ := net.ParseCIDR("2001:db8::/62")
	alloc, _ := NewBitmapAllocator(*prefix, 64)

	allocd := []net.IPNet{}
	for i := 0; i < 4; i++ {
		net, err := alloc.Allocate(net.IPNet{Mask: net.CIDRMask(64, 128)})
		if err != nil {
			t.Fatalf("Error before exhaustion: %v", err)
		}
		allocd = append(allocd, net)
	}

	_, err := alloc.Allocate(net.IPNet{})
	if err == nil {
		t.Fatalf("Successfully allocated more prefixes than there are in the pool")
	}

	err = alloc.Free(allocd[1])
	if err != nil {
		t.Fatalf("Could not free: %v", err)
	}
	net, err := alloc.Allocate(allocd[1])
	if err != nil {
		t.Fatalf("Could not reallocate after free: %v", err)
	}
	if !net.IP.Equal(allocd[1].IP) || net.Mask.String() != allocd[1].Mask.String() {
		t.Fatalf("Did not obtain the right network after free: got %v, expected %v", net, allocd[1])
	}

}

func TestOutOfPool(t *testing.T) {
	alloc := getAllocator()
	_, prefix, _ := net.ParseCIDR("fe80:abcd::/48")

	res, err := alloc.Allocate(*prefix)
	if err != nil {
		t.Fatalf("Failed to allocate with invalid hint: %v", err)
	}
	if !alloc.containing.Contains(res.IP) {
		t.Fatal("Obtained prefix outside of range: ", res)
	}
	if prefLen, totalLen := res.Mask.Size(); prefLen != 64 || totalLen != 128 {
		t.Fatalf("Prefixes have wrong size %d/%d", prefLen, totalLen)
	}
}
