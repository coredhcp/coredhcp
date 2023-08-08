// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// This allocator only returns prefixes of a single size
// This is much simpler to implement (reduces the problem to an equivalent of
// single ip allocations), probably makes sense in cases where the available
// range is much larger than the expected number of clients. Also is what KEA
// does so at least it's not worse than that

package bitmap

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/bits-and-blooms/bitset"

	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins/allocators"
)

var log = logger.GetLogger("plugins/allocators/bitmap")

// Allocator is a prefix allocator allocating in chunks of a fixed size
// regardless of the size requested by the client.
// It consumes an amount of memory proportional to the total amount of available prefixes
type Allocator struct {
	containing net.IPNet
	page       int
	bitmap     *bitset.BitSet
	l          sync.Mutex
}

// prefix must verify: containing.Mask.Size < prefix.Mask.Size < page
func (a *Allocator) toIndex(base net.IP) (uint, error) {
	value, err := allocators.Offset(base, a.containing.IP, a.page)
	if err != nil {
		return 0, fmt.Errorf("Cannot compute prefix index: %w", err)
	}

	return uint(value), nil
}

func (a *Allocator) toPrefix(idx uint) (net.IP, error) {
	return allocators.AddPrefixes(a.containing.IP, uint64(idx), uint64(a.page))
}

// Allocate reserves a maxsize-sized block and returns a block of size
// min(maxsize, hint.size)
func (a *Allocator) Allocate(hint net.IPNet) (ret net.IPNet, err error) {

	// Ensure size is max(maxsize, hint.size)
	reqSize, hintErr := hint.Mask.Size()
	if reqSize < a.page || hintErr != 128 {
		reqSize = a.page
	}
	ret.Mask = net.CIDRMask(reqSize, 128)

	// Try to allocate the requested prefix
	a.l.Lock()
	defer a.l.Unlock()
	if hint.IP.To16() != nil && a.containing.Contains(hint.IP) {
		idx, hintErr := a.toIndex(hint.IP)
		if hintErr == nil && !a.bitmap.Test(idx) {
			a.bitmap.Set(idx)
			ret.IP, err = a.toPrefix(idx)
			return
		}
	}

	// Find a free prefix
	next, ok := a.bitmap.NextClear(0)
	if !ok {
		err = allocators.ErrNoAddrAvail
		return
	}
	a.bitmap.Set(next)
	ret.IP, err = a.toPrefix(next)
	if err != nil {
		// This violates the assumption that every index in the bitmap maps back to a valid prefix
		err = fmt.Errorf("BUG: could not get prefix from allocation: %w", err)
		a.bitmap.Clear(next)
	}
	return
}

// Free returns the given prefix to the available pool if it was taken.
func (a *Allocator) Free(prefix net.IPNet) error {
	idx, err := a.toIndex(prefix.IP.Mask(prefix.Mask))
	if err != nil {
		return fmt.Errorf("Could not find prefix in pool: %w", err)
	}

	a.l.Lock()
	defer a.l.Unlock()

	if !a.bitmap.Test(idx) {
		return &allocators.ErrDoubleFree{Loc: prefix}
	}
	a.bitmap.Clear(idx)
	return nil
}

// NewBitmapAllocator creates a new allocator, allocating /`size` prefixes
// carved out of the given `pool` prefix
func NewBitmapAllocator(pool net.IPNet, size int) (*Allocator, error) {

	poolSize, _ := pool.Mask.Size()
	allocOrder := size - poolSize

	if allocOrder < 0 {
		return nil, errors.New("The size of allocated prefixes cannot be larger than the pool they're allocated from")
	} else if allocOrder >= strconv.IntSize {
		return nil, fmt.Errorf("A pool with more than 2^%d items is not representable", size-poolSize)
	} else if allocOrder >= 32 {
		log.Warningln("Using a pool of more than 2^32 elements may result in large memory consumption")
	}

	if !(1<<uint(allocOrder) <= bitset.Cap()) {
		return nil, errors.New("Can't fit this pool using the bitmap allocator")
	}

	alloc := Allocator{
		containing: pool,
		page:       size,

		bitmap: bitset.New(1 << uint(allocOrder)),
	}

	return &alloc, nil
}
