/*
   conflux - Distributed database synchronization library
	Based on the algorithm described in
		"Set Reconciliation with Nearly Optimal	Communication Complexity",
			Yaron Minsky, Ari Trachtenberg, and Richard Zippel, 2004.

   Copyright (c) 2012-2015  Casey Marshall <cmars@cmarstech.com>

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package conflux

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

// P_128 defines a finite field Z(P) that includes all 128-bit integers.
var P_128 = big.NewInt(0).SetBytes([]byte{
	0x1, 0x11, 0xd, 0xb2, 0x97, 0xcd, 0x30, 0x8d,
	0x90, 0xe5, 0x3f, 0xb8, 0xa1, 0x30, 0x90, 0x97, 0xe9})

// P_160 defines a finite field Z(P) that includes all 160-bit integers.
var P_160 = big.NewInt(0).SetBytes([]byte{
	0x1, 0xfe, 0x90, 0xe7, 0xb4, 0x19, 0x88, 0xa6,
	0x41, 0xb1, 0xa6, 0xfe, 0xc8, 0x7d, 0x89, 0xa3,
	0x1e, 0x2a, 0x61, 0x31, 0xf5})

// P_256 defines a finite field Z(P) that includes all 256-bit integers.
var P_256 = big.NewInt(0).SetBytes([]byte{
	0x1, 0xdd, 0xf4, 0x8a, 0xc3, 0x45, 0x19, 0x18,
	0x13, 0xab, 0x7d, 0x92, 0x27, 0x99, 0xe8, 0x93,
	0x96, 0x19, 0x43, 0x8, 0xa4, 0xa5, 0x9, 0xb,
	0x36, 0xc9, 0x62, 0xd5, 0xd5, 0xd6, 0xdd, 0x80, 0x27})

// P_512 defines a finite field Z(P) that includes all 512-bit integers.
var P_512 = big.NewInt(0).SetBytes([]byte{
	0x1, 0xc7, 0x19, 0x72, 0x25, 0xf4, 0xa5, 0xd5,
	0x8a, 0xc0, 0x2, 0xa4, 0xdc, 0x8d, 0xb1, 0xd9,
	0xb0, 0xa1, 0x5b, 0x7a, 0x43, 0x22, 0x5d, 0x5b,
	0x51, 0xa8, 0x1c, 0x76, 0x17, 0x44, 0x2a, 0x4a,
	0x9c, 0x62, 0xdc, 0x9e, 0x25, 0xd6, 0xe3, 0x12,
	0x1a, 0xea, 0xef, 0xac, 0xd9, 0xfd, 0x8d, 0x6c,
	0xb7, 0x26, 0x6d, 0x19, 0x15, 0x53, 0xd7, 0xd,
	0xb6, 0x68, 0x3b, 0x65, 0x40, 0x89, 0x18, 0x3e, 0xbd})

// P_SKS is the finite field used by SKS, the Synchronizing Key Server.
var P_SKS *big.Int

var zero = big.NewInt(0)

func init() {
	P_SKS, _ = big.NewInt(0).SetString("530512889551602322505127520352579437339", 10)
}

// Zp represents a value in the finite field Z(p), an integer in which all
// arithmetic is (mod p).
type Zp struct {

	// i is the integer's value.
	i big.Int

	// p is the prime bound of the finite field Z(p).
	p *big.Int
}

// Z returns an integer in the finite field P initialized to 0.
func Z(p *big.Int) *Zp {
	return &Zp{p: p}
}

// Zzp returns an integer in the finite field P initialized to zp.
func Zzp(zp *Zp) *Zp {
	zzp := &Zp{p: zp.p}
	zzp.i.Set(&zp.i)
	return zzp
}

// Zi returns an integer n in the finite field p.
func Zi(p *big.Int, n int) *Zp {
	zp := &Zp{p: p}
	zp.i.SetInt64(int64(n))
	zp.Norm()
	return zp
}

// Zb returns an integer in the finite field p from a byte representation.
func Zb(p *big.Int, b []byte) *Zp {
	z := Zi(p, 0)
	z.SetBytes(b)
	return z
}

// Zs returns an integer from base10 string s in the finite field p.
func Zs(p *big.Int, s string) *Zp {
	zp := &Zp{p: p}
	_, ok := zp.i.SetString(s, 10)
	if !ok {
		panic(fmt.Sprintf("invalid integer %q", s))
	}
	zp.Norm()
	return zp
}

func randbits(nbits int) *big.Int {
	nbytes := nbits / 8
	if nbits%8 != 0 {
		nbytes++
	}
	rstring := make([]byte, nbytes)
	rand.Reader.Read(rstring)
	var rval, high, big2, exp, rem big.Int
	rval.SetBytes(rstring)
	big2.SetInt64(int64(2))
	exp.SetInt64(int64(nbits - 1))
	high.Exp(&big2, &exp, nil)
	rem.Mod(&rval, &high)
	rval.Add(&high, &rem)
	return &rval
}

func randint(high *big.Int) *big.Int {
	nbits := high.BitLen()
	nbytes := nbits / 8
	if nbits%8 != 0 {
		nbytes++
	}
	rstring := make([]byte, nbytes)
	rand.Reader.Read(rstring)
	var rval big.Int
	rval.SetBytes(rstring)
	rval.Mod(&rval, high)
	return &rval
}

// Zrand returns a random integer in the finite field p.
func Zrand(p *big.Int) *Zp {
	zp := &Zp{p: p}
	zp.i.Set(randint(p))
	return zp
}

// Zarray returns a new array of integers, all initialized to v.
func Zarray(p *big.Int, n int, v *Zp) []Zp {
	result := make([]Zp, n)
	for i := 0; i < n; i++ {
		result[i].Set(v)
	}
	return result
}

func reversed(b []byte) []byte {
	l := len(b)
	result := make([]byte, l)
	for i := 0; i < l; i++ {
		result[i] = b[l-i-1]
	}
	return result
}

// P returns the modulus of Zp.
func (zp *Zp) P() *big.Int {
	return zp.p
}

// FullKeyHash returns Zp in the format of a full-key hash.
func (zp *Zp) FullKeyHash() string {
	return hex.EncodeToString(zp.Bytes())
}

// Bytes returns the byte representation of Zp.
func (zp *Zp) Bytes() []byte {
	return reversed(zp.i.Bytes())
}

// Set sets zp to x and returns zp.
func (zp *Zp) Set(x *Zp) *Zp {
	zp.p = x.p
	zp.i.Set(&x.i)
	return zp
}

// SetBytes sets the integer from its byte representation.
func (zp *Zp) SetBytes(b []byte) {
	zp.i.SetBytes(reversed(b))
	zp.Norm()
}

// Copy returns a new Zp instance with the same value.
func (zp *Zp) Copy() *Zp {
	return Zzp(zp)
}

// Norm normalizes the integer to its finite field, (mod P).
func (zp *Zp) Norm() *Zp {
	zp.i.Mod(&zp.i, zp.p)
	return zp
}

// Cmp compares zp with another integer. See big.Int.Cmp for return value
// semantics.
func (zp *Zp) Cmp(x *Zp) int {
	zp.assertEqualP(x)
	return (&zp.i).Cmp(&x.i)
}

// IsZero returns whether the integer is zero.
func (zp *Zp) IsZero() bool {
	return zp.i.Cmp(zero) == 0
}

// Add sets the integer value to the sum of two integers, returning the result.
func (zp *Zp) Add(x, y *Zp) *Zp {
	x.assertP(y.p)
	zp.p = x.p
	zp.i.Add(&x.i, &y.i)
	zp.Norm()
	return zp
}

// Sub sets the integer value to the difference of two integers, returning the result.
func (zp *Zp) Sub(x, y *Zp) *Zp {
	x.assertP(y.p)
	zp.p = x.p
	zp.i.Sub(&x.i, &y.i)
	zp.Norm()
	return zp
}

// Mul sets the integer value to the product of two integers, returning the result.
func (zp *Zp) Mul(x, y *Zp) *Zp {
	x.assertP(y.p)
	zp.p = x.p
	zp.i.Mul(&x.i, &y.i)
	zp.Norm()
	return zp
}

// Inv sets the integer value to its multiplicative inverse, returning the result.
func (zp *Zp) Inv() *Zp {
	zp.i.ModInverse(&zp.i, zp.p)
	return zp
}

// Exp sets the integer value to x**y ("x to the yth power"), returning the
// result.
func (zp *Zp) Exp(x, y *Zp) *Zp {
	x.assertP(y.p)
	zp.p = x.p
	zp.i.Exp(&x.i, &y.i, zp.p)
	return zp
}

// Div sets the integer value to x/y in the finite field p, returning the result.
func (zp *Zp) Div(x, y *Zp) *Zp {
	return zp.Mul(x, y.Copy().Inv())
}

// Neg sets the integer to its additive inverse, returning the result.
func (zp *Zp) Neg() *Zp {
	zp.i.Sub(zp.p, &zp.i)
	zp.Norm()
	return zp
}

func (zp *Zp) String() string {
	return zp.i.String()
}

func (zp *Zp) Int64() int64 {
	return zp.i.Int64()
}

// In sets the finite field to P on an uninitialized Zp, otherwise it asserts
// P.
func (zp *Zp) In(p *big.Int) *Zp {
	if zp.p == nil {
		zp.p = p
	} else {
		zp.assertP(p)
	}
	return zp
}

// assertP asserts an integer is in the expected finite field P.
func (zp *Zp) assertP(p *big.Int) {
	if zp.p.Cmp(p) != 0 {
		panic(fmt.Sprintf("expect finite field Z(%v), was Z(%v)", p, zp.p))
	}
}

// assertEqualP asserts all integers share the same finite field P as this one.
func (zp *Zp) assertEqualP(values ...*Zp) {
	for _, v := range values {
		zp.assertP(v.p)
	}
}

// ZSet is a set of integers in a finite field.
type ZSet struct {
	s map[string]*big.Int
	p *big.Int
}

// NewZSet returns a new ZSet containing the given elements.
func NewZSet(elements ...*Zp) *ZSet {
	zs := &ZSet{s: make(map[string]*big.Int, len(elements))}
	for i := range elements {
		zs.Add(elements[i])
	}
	return zs
}

func NewZSetSlice(elements []Zp) *ZSet {
	zs := &ZSet{s: make(map[string]*big.Int, len(elements))}
	for i := range elements {
		zs.Add(&elements[i])
	}
	return zs
}

// Len returns the length of the set.
func (zs *ZSet) Len() int {
	if zs == nil || zs.s == nil {
		return 0
	}
	return len(zs.s)
}

// Add adds an element to the set.
func (zs *ZSet) Add(v *Zp) {
	if zs.p == nil {
		zs.p = v.p
	} else {
		v.assertP(zs.p)
	}
	zs.s[v.String()] = big.NewInt(0).Set(&v.i)
}

// Remove removes an element from the set.
func (zs *ZSet) Remove(v *Zp) {
	delete(zs.s, v.String())
}

// Contains returns whether the set contains the given element as a member.
func (zs *ZSet) Contains(v *Zp) bool {
	_, ok := zs.s[v.String()]
	return ok
}

// Equal returns whether this set is equal to another set.
func (zs *ZSet) Equal(other *ZSet) bool {
	if len(zs.s) != len(other.s) {
		return false
	}
	for k, _ := range zs.s {
		_, has := other.s[k]
		if !has {
			return false
		}
	}
	return true
}

// AddSlice adds all the given elements to the set.
func (zs *ZSet) AddSlice(other []Zp) {
	for i := range other {
		zs.Add(&other[i])
	}
}

// AddAll adds all elements in another set.
func (zs *ZSet) AddAll(other *ZSet) {
	if zs.p == nil {
		zs.p = other.p
	}
	for k, v := range other.s {
		zs.s[k] = v
	}
}

// RemoveSlice removes all the given elements from the set.
func (zs *ZSet) RemoveSlice(other []Zp) {
	for i := range other {
		zs.Remove(&other[i])
	}
}

// RemoveAll removes all elements in another set from this one.
func (zs *ZSet) RemoveAll(other *ZSet) {
	if zs.p == nil {
		zs.p = other.p
	}
	for k, _ := range other.s {
		delete(zs.s, k)
	}
}

// Items returns a slice of all elements in the set.
func (zs *ZSet) Items() []Zp {
	if zs == nil {
		return nil
	}
	result := make([]Zp, len(zs.s))
	i := 0
	for _, v := range zs.s {
		result[i] = Zp{p: zs.p}
		result[i].i.Set(v)
		i++
	}
	return result
}

// String returns a string representation of the set.
func (zs *ZSet) String() string {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "{")
	first := true
	for k, _ := range zs.s {
		if first {
			first = false
		} else {
			fmt.Fprintf(buf, ", ")
		}
		fmt.Fprintf(buf, "%v", k)
	}
	fmt.Fprintf(buf, "}")
	return string(buf.Bytes())
}

// ZpSlice is a collection of integers in a finite field.
type ZpSlice []Zp

// String returns a string representation of the ZpSlice.
func (zp ZpSlice) String() string {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "{")
	first := true
	for k, _ := range zp {
		if first {
			first = false
		} else {
			fmt.Fprintf(buf, ", ")
		}
		fmt.Fprintf(buf, "%v", k)
	}
	fmt.Fprintf(buf, "}")
	return string(buf.Bytes())
}

// ZSetDiff returns the set difference between two ZSets:
// the set of all Z(p) in a that are not in b.
func ZSetDiff(a *ZSet, b *ZSet) *ZSet {
	result := NewZSet()
	if a.p != nil {
		result.p = a.p
	} else if b.p != nil {
		result.p = b.p
	}
	for k, v := range a.s {
		_, has := b.s[k]
		if !has {
			result.s[k] = v
		}
	}
	return result
}
