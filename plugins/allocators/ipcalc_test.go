// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package allocators

import (
	"fmt"
	"net"
	"testing"

	"math/rand"
)

func ExampleOffset() {
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 0))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 16))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 32))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 48))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 64))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 73))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 80))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 96))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 112))
	fmt.Println(Offset(net.ParseIP("2001:db8:0:aabb::"), net.ParseIP("2001:db8:ff::34"), 128))
	// Output:
	// 0 <nil>
	// 0 <nil>
	// 0 <nil>
	// 254 <nil>
	// 16667973 <nil>
	// 8534002176 <nil>
	// 1092352278528 <nil>
	// 71588398925611008 <nil>
	// 0 Operation overflows
	// 0 Operation overflows
}

func ExampleAddPrefixes() {
	fmt.Println(AddPrefixes(net.ParseIP("2001:db8::"), 0xff, 64))
	fmt.Println(AddPrefixes(net.ParseIP("2001:db8::"), 0x1, 128))
	fmt.Println(AddPrefixes(net.ParseIP("2001:db8::"), 0xff, 32))
	fmt.Println(AddPrefixes(net.ParseIP("2001:db8::"), 0x1, 16))
	fmt.Println(AddPrefixes(net.ParseIP("2001:db8::"), 0xff, 65))
	// Error cases
	fmt.Println(AddPrefixes(net.ParseIP("2001:db8::"), 0xff, 8))
	fmt.Println(AddPrefixes(net.IP{10, 0, 0, 1}, 64, 32))
	// Output:
	// 2001:db8:0:ff:: <nil>
	// 2001:db8::1 <nil>
	// 2001:eb7:: <nil>
	// 2002:db8:: <nil>
	// 2001:db8:0:7f:8000:: <nil>
	// <nil> Operation overflows
	// <nil> AddPrefixes needs 128-bit IPs
}

// Offset is used as a hash function, so it needs to be reasonably fast
func BenchmarkOffset(b *testing.B) {
	// Need predictable randomness for benchmark reproducibility
	rng := rand.New(rand.NewSource(0))
	addresses := make([]byte, b.N*net.IPv6len*2)
	_, err := rng.Read(addresses)
	if err != nil {
		b.Fatalf("Could not generate random addresses: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// The arrays will be in cache, so this should amortize to measure mostly just the offset
		// computation itself
		_, _ = Offset(
			addresses[i*2*net.IPv6len:(i*2+1)*net.IPv6len],
			addresses[(i*2+1)*net.IPv6len:(i+1)*2*net.IPv6len],
			(i*4)%128,
		)
	}
}
