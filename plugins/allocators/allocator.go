// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package allocators provides the interface and the algorithm(s) for allocation of ipv6
// prefixes of various sizes within a larger prefix.
// There are many many parallels with memory allocation.
package allocators

import (
	"errors"
	"fmt"
	"net"
)

// Allocator is the interface to the address allocator. It only finds and
// allocates blocks and is not concerned with lease-specific questions like
// expiration (ie garbage collection needs to be handled separately)
type Allocator interface {
	// Allocate finds a suitable prefix of the given size and returns it.
	//
	// hint is a prefix, which the client desires especially, and that the
	// allocator MAY try to return; the allocator SHOULD try to return a prefix of
	// the same size as the given hint prefix. The allocator MUST NOT return an
	// error if a prefix was successfully assigned, even if the prefix has nothing
	// in common with the hinted prefix
	Allocate(hint net.IPNet) (net.IPNet, error)

	// Free returns the prefix containing the given network to the pool
	//
	// Free may return a DoubleFreeError if the prefix being returned was not
	// previously allocated
	Free(net.IPNet) error
}

// ErrDoubleFree is an error type returned by Allocator.Free() when a
// non-allocated block is passed
type ErrDoubleFree struct {
	Loc net.IPNet
}

// String returns a human-readable error message for a DoubleFree error
func (err *ErrDoubleFree) Error() string {
	return fmt.Sprint("Attempted to free unallocated block at ", err.Loc.String())
}

// ErrNoAddrAvail is returned when we can't allocate an IP because there's no unallocated space left
var ErrNoAddrAvail = errors.New("no address available to allocate")
