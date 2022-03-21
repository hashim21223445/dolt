// Copyright 2022 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/dolthub/dolt/go/gen/fb/serial"
	"github.com/dolthub/dolt/go/store/hash"
)

type SerialMessage []byte

func (sm SerialMessage) Kind() NomsKind {
	return SerialMessageKind
}

func (sm SerialMessage) Value(ctx context.Context) (Value, error) {
	return sm, nil
}

func (sm SerialMessage) isPrimitive() bool {
	return true
}

func (sm SerialMessage) Equals(other Value) bool {
	if other.Kind() != SerialMessageKind {
		return false
	}
	return bytes.Equal(sm, other.(SerialMessage))
}

func (sm SerialMessage) Hash(nbf *NomsBinFormat) (hash.Hash, error) {
	return getHash(sm, nbf)
}

func (sm SerialMessage) HumanReadableString() string {
	return "SerialMessage"
}

func (sm SerialMessage) Less(nbf *NomsBinFormat, other LesserValuable) (bool, error) {
	if v2, ok := other.(SerialMessage); ok {
		return bytes.Compare(sm, v2) == -1, nil
	}
	return sm.Kind() < other.Kind(), nil
}

func (sm SerialMessage) WalkValues(ctx context.Context, cb ValueCallback) error {
	return errors.New("unsupported WalkValues on SerialMessage. Use types.WalkValues.")
}

// Refs in SerialMessage do not have height.
const serialMessageRefHeight = 5

func (sm SerialMessage) WalkRefs(nbf *NomsBinFormat, cb RefCallback) error {
	switch serial.GetFileID([]byte(sm)) {
	case serial.StoreRootFileID:
		msg := serial.GetRootAsStoreRoot([]byte(sm), 0)
		rm := msg.Refs(nil)
		refs := rm.RefArrayBytes()
		for i := 0; i < rm.NamesLength(); i++ {
			off := i * 20
			addr := hash.New(refs[off : off+20])
			r, err := constructRef(nbf, addr, PrimitiveTypeMap[ValueKind], 5)
			if err != nil {
				return err
			}
			if err = cb(r); err != nil {
				return err
			}
		}
	case serial.TagFileID:
		msg := serial.GetRootAsTag([]byte(sm), 0)
		addr := hash.New(msg.CommitAddrBytes())
		r, err := constructRef(nbf, addr, PrimitiveTypeMap[ValueKind], 5)
		if err != nil {
			return err
		}
		return cb(r)
	case serial.WorkingSetFileID:
		msg := serial.GetRootAsWorkingSet([]byte(sm), 0)
		addr := hash.New(msg.WorkingRootAddrBytes())
		r, err := constructRef(nbf, addr, PrimitiveTypeMap[ValueKind], 5)
		if err != nil {
			return err
		}
		if err = cb(r); err != nil {
			return err
		}
		if msg.StagedRootAddrLength() != 0 {
			addr = hash.New(msg.StagedRootAddrBytes())
			r, err = constructRef(nbf, addr, PrimitiveTypeMap[ValueKind], 5)
			if err != nil {
				return err
			}
			if err = cb(r); err != nil {
				return err
			}
		}
		if msg.MergeStateAddrLength() != 0 {
			addr = hash.New(msg.MergeStateAddrBytes())
			r, err = constructRef(nbf, addr, PrimitiveTypeMap[ValueKind], 5)
			if err != nil {
				return err
			}
			if err = cb(r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (sm SerialMessage) readFrom(nbf *NomsBinFormat, b *binaryNomsReader) (Value, error) {
	bytes := b.ReadInlineBlob()
	return SerialMessage(bytes), nil
}

func (sm SerialMessage) skip(nbf *NomsBinFormat, b *binaryNomsReader) {
	size := uint32(b.readUint16())
	b.skipBytes(size)
}

func (sm SerialMessage) typeOf() (*Type, error) {
	return PrimitiveTypeMap[SerialMessageKind], nil
}

func (sm SerialMessage) writeTo(w nomsWriter, nbf *NomsBinFormat) error {
	byteLen := len(sm)
	if byteLen > math.MaxUint16 {
		return fmt.Errorf("SerialMessage has length %v when max is %v", byteLen, math.MaxUint16)
	}

	err := SerialMessageKind.writeTo(w, nbf)
	if err != nil {
		return err
	}
	w.writeUint16(uint16(byteLen))
	w.writeRaw(sm)
	return nil
}