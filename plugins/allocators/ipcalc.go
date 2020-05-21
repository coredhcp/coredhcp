// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Provides functions to add/subtract ipv6 addresses, for use in offset
// calculations in allocators

package allocators

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/bits"
	"net"
)

// ErrOverflow is returned when arithmetic operations on IPs carry bits
// over/under the 0th or 128th bit respectively
var ErrOverflow = errors.New("Operation overflows")

// Offset returns the absolute distance between addresses `a` and `b` in units
// of /`prefixLength` subnets.
// Both addresses will have a /`prefixLength` mask applied to them, any
// differences of less than that will be discarded
// If the distance is larger than 2^64 units of /`prefixLength` an error is returned
//
// This function is used in allocators to index bitmaps by an offset from the
// first ip of the range
func Offset(a, b net.IP, prefixLength int) (uint64, error) {
	if prefixLength > 128 || prefixLength < 0 {
		return 0, errors.New("prefix out of range")
	}

	reverse := bytes.Compare(a, b)
	if reverse == 0 {
		return 0, nil
	} else if reverse < 0 {
		a, b = b, a
	}

	// take an example of [a:b:c:d:e:f:g:h] [1:2:3:4:5:6:7:8]
	// Cut the addresses as such: [a:b:c:d|e:f:g:h] [1:2:3:4|5:6:7:8] so we can use
	// native integers for computation
	ah, bh := binary.BigEndian.Uint64(a[:8]), binary.BigEndian.Uint64(b[:8])

	if prefixLength <= 64 {
		// [(a:b:c):d|e:f:g:h] - [(1:2:3):4|5:6:7:8]
		// Only the high bits matter, so the distance always fits within 64 bits.
		// We shift to remove anything to the right of the cut
		// [(a:b:c):d] => [0:a:b:c]
		return (ah - bh) >> (64 - uint(prefixLength)), nil
	}

	// General case where both high and low bits matter
	al, bl := binary.BigEndian.Uint64(a[8:]), binary.BigEndian.Uint64(b[8:])
	distanceLow, borrow := bits.Sub64(al, bl, 0)

	// This is the distance between the high bits. depending on the prefix unit, we
	// will shift this distance left or right
	distanceHigh, _ := bits.Sub64(ah, bh, borrow) // [a:b:c:d] - [1:2:3:4]

	// [a:b:c:(d|e:f:g):h] - [1:2:3:(4|5:6:7):8]
	// we cut in the low bits (eg. between the parentheses)
	// To ensure we stay within 64 bits, we need to ensure [a:b:c:d] - [1:2:3:4] = [0:0:0:d-4]
	// so that we don't overflow when adding to the low bits
	if distanceHigh >= (1 << (128 - uint(prefixLength))) {
		return 0, ErrOverflow
	}

	// Schema of the carry and shifts:
	// [a:b:c:(d]
	//          [e:f:g):h]
	// <--------------->   prefixLen
	//                 <-> 128 - prefixLen (cut right)
	// <----->             prefixLen - 64 (cut left)
	//
	// [a:b:c:(d] => [d:0:0:0]
	distanceHigh <<= uint(prefixLength) - 64
	// [e:f:g):h] => [0:e:f:g]
	distanceLow >>= 128 - uint(prefixLength)
	// [d:0:0:0] + [0:e:f:g] = (d:e:f:g)
	return distanceHigh + distanceLow, nil
}

// AddPrefixes returns the `n`th /`unit` subnet after the `ip` base subnet. It
// is the converse operation of Offset(), used to retrieve a prefix from the
// index within the allocator table
func AddPrefixes(ip net.IP, n, unit uint64) (net.IP, error) {
	if unit == 0 && n != 0 {
		return net.IP{}, ErrOverflow
	} else if n == 0 {
		return ip, nil
	}
	if len(ip) != 16 {
		// We don't actually care if they're true v6 or v4-mapped,
		// but they need to be 128-bit to handle as 64-bit ints
		return net.IP{}, errors.New("AddPrefixes needs 128-bit IPs")
	}

	// Compute as pairs of uint64 for easier operations
	// This could all be 1 function call if go had 128-bit integers
	iph, ipl := binary.BigEndian.Uint64(ip[:8]), binary.BigEndian.Uint64(ip[8:])

	// Compute `n` /`unit` subnets as uint64 pair
	var offh, offl uint64
	if unit <= 64 {
		offh = n << (64 - unit)
	} else {
		offh, offl = bits.Mul64(n, 1<<(128-unit))
	}

	// Now add the 2, check for overflow
	ipl, carry := bits.Add64(offl, ipl, 0)
	iph, carry = bits.Add64(offh, iph, carry)
	if carry != 0 {
		return net.IP{}, ErrOverflow
	}

	// Finally convert back to net.IP
	ret := make(net.IP, net.IPv6len)
	binary.BigEndian.PutUint64(ret[:8], iph)
	binary.BigEndian.PutUint64(ret[8:], ipl)

	return ret, nil
}
