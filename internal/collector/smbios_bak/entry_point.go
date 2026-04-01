package smbios

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func calChecksum(data []byte, skipIndex int) uint8 {
	var sum uint8
	for i, b := range data {
		if i == skipIndex {
			continue
		}
		sum += b
	}
	return uint8(0x100 - int(sum))
}

type entryPoint32 struct {
	AnchorString             [4]uint8
	Checksum                 uint8
	Length                   uint8
	MajorVersion             uint8
	MinorVersion             uint8
	MaximumStructureSize     uint16
	Revision                 uint8
	FormattedArea            [5]uint8
	IntermediateAnchorString [5]uint8
	IntermediateChecksum     uint8
	TableLength              uint16
	TableAddress             uint32
	NumberOfStructures       uint16
	BCDRevision              uint8
}

// Table returns table address and length.
func (ep *entryPoint32) Table() (int, int) {
	return int(ep.TableAddress), int(ep.TableLength)
}

func (ep *entryPoint32) MarshalBinary() ([]byte, error) {
	var bf bytes.Buffer
	if err := binary.Write(&bf, binary.LittleEndian, ep); err != nil {
		return nil, err
	}

	return bf.Bytes(), nil
}

func (ep *entryPoint32) UnmarshalBinary(data []byte) error {
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, ep); err != nil {
		return fmt.Errorf("read binary 32: %w", err)
	}

	if !bytes.Equal(ep.AnchorString[:], []byte(anchor32)) {
		return fmt.Errorf("invalid anchor string %v", ep.AnchorString[:])
	}

	if ep.Length != anchor32Len {
		return fmt.Errorf("invalid entry point length %d", ep.Length)
	}

	if ep.Checksum != calChecksum(data, 4) {
		return fmt.Errorf("invalid entry point checksum %d", ep.Checksum)
	}

	if !bytes.Equal(ep.IntermediateAnchorString[:], []byte("_DMI_")) {
		return fmt.Errorf("invalid intermediate anchor string %v", ep.IntermediateAnchorString[:])
	}

	if ep.IntermediateChecksum != calChecksum(data[0x10:0x1F], 5) {
		return fmt.Errorf("invalid intermediate checksum %d", ep.IntermediateChecksum)
	}

	return nil
}

type entryPoint64 struct {
	AnchorString          [5]uint8
	Checksum              uint8
	Length                uint8
	MajorVersion          uint8
	MinorVersion          uint8
	DocumentationRevision uint8
	Revision              uint8
	Reserved              uint8
	MaximumStructureSize  uint16
	TableAddress          uint64
}

func (ep *entryPoint64) Table() (int, int) {
	return int(ep.TableAddress), int(ep.MaximumStructureSize)
}

func (ep *entryPoint64) MarshalBinary() ([]byte, error) {
	var bf bytes.Buffer
	if err := binary.Write(&bf, binary.LittleEndian, ep); err != nil {
		return nil, err
	}

	return bf.Bytes(), nil
}

func (ep *entryPoint64) UnmarshalBinary(data []byte) error {
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, ep); err != nil {
		return fmt.Errorf("read binary: %w", err)
	}

	if !bytes.Equal(ep.AnchorString[:], []byte(anchor64)) {
		return fmt.Errorf("invalid anchor string %v", ep.AnchorString[:])
	}

	if ep.Length != anchor64Len {
		return fmt.Errorf("invalid entry point length %d", ep.Length)
	}

	if ep.Checksum != calChecksum(data, 5) {
		return fmt.Errorf("invalid entry point checksum %d", ep.Checksum)
	}

	return nil
}
