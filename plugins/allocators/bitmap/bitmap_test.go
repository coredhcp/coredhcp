// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package bitmap

import (
	"math"
	"math/rand"
	"net"
	"testing"

	"github.com/bits-and-blooms/bitset"
)

func getAllocator(bits int) *Allocator {
	_, prefix, err := net.ParseCIDR("2001:db8::/56")
	if err != nil {
		panic(err)
	}
	alloc, err := NewBitmapAllocator(*prefix, 56+bits)
	if err != nil {
		panic(err)
	}

	return alloc
}
func TestAlloc(t *testing.T) {
	alloc := getAllocator(8)

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
	alloc := getAllocator(8)
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

func prefixSizeForAllocs(allocs int) int {
	return int(math.Ceil(math.Log2(float64(allocs))))
}

// Benchmark parallel Allocate, when the bitmap is mostly empty and we're allocating few values
// compared to the available allocations
func BenchmarkParallelAllocInitiallyEmpty(b *testing.B) {
	// Run with -race to debug concurrency issues

	alloc := getAllocator(prefixSizeForAllocs(b.N) + 2) // Use max 25% of the bitmap (initially empty)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if net, err := alloc.Allocate(net.IPNet{}); err != nil {
				b.Logf("Could not allocate (got %v and an error): %v", net, err)
				b.Fail()
			}
		}
	})
}

func BenchmarkParallelAllocPartiallyFilled(b *testing.B) {
	// We'll make a bitmap with 2x the number of allocs we want to make.
	// Then randomly fill it to about 50% utilization
	alloc := getAllocator(prefixSizeForAllocs(b.N) + 1)

	// Build a replacement bitmap that we'll put in the allocator, with approx. 50% of values filled
	newbmap := make([]uint64, alloc.bitmap.Len())
	for i := uint(0); i < alloc.bitmap.Len(); i++ {
		newbmap[i] = rand.Uint64()
	}
	alloc.bitmap = bitset.From(newbmap)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if net, err := alloc.Allocate(net.IPNet{}); err != nil {
				b.Logf("Could not allocate (got %v and an error): %v", net, err)
				b.Fail()
			}
		}
	})
}
