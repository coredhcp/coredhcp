// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package allocators

import (
	"fmt"
	"net"
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
	// Output:
	// 2001:db8:0:ff:: <nil>
	// 2001:db8::1 <nil>
	// 2001:eb7:: <nil>
	// 2002:db8:: <nil>
	// 2001:db8:0:7f:8000:: <nil>
}
