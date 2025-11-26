package seektable

import (
	"encoding/binary"
	"errors"
	"io"
)

const headerMagicNumber uint32 = 0x184D2A5E
const footerMagicNumber uint32 = 0x8F92EAB1

var ErrInvalidSeekTable = errors.New("invalid seek table")
var ErrInvalidSeekTableFooterMagicNumber = errors.New("invalid seek table footer magic number")
var ErrInvalidSeekTableHeaderMagicNumber = errors.New("invalid seek table header magic number")
var ErrSeekTableSizeMismatch = errors.New("seek table size mismatch")

func ReadTableFromReadSeeker(data io.ReadSeeker) (*Table, error) {
	// Get last 9 bytes to read footer
	_, err := data.Seek(-9, io.SeekEnd)
	if err != nil {
		return nil, errors.Join(errors.New("error while seeking to seek table footer"), err)
	}
	var footer [9]byte
	_, err = io.ReadFull(data, footer[:])
	if err != nil {
		return nil, errors.Join(errors.New("error while reading seek table footer"), err)
	}

	numEntries := binary.LittleEndian.Uint32(footer[0:4])
	if binary.LittleEndian.Uint32(footer[5:9]) != footerMagicNumber {
		return nil, errors.Join(ErrInvalidSeekTable, ErrInvalidSeekTableFooterMagicNumber)
	}

	// Seek to the beginning of the seek table. 8 bytes header + (entries * 8 bytes each) + 9 bytes footer
	seekTableSize := int64(8 + (numEntries * 8) + 9)
	_, err = data.Seek(-seekTableSize, io.SeekEnd)
	if err != nil {
		return nil, errors.Join(errors.New("error while seeking to seek table start"), err)
	}

	// Read and validate header
	var header [8]byte
	_, err = io.ReadFull(data, header[:])
	if err != nil {
		return nil, errors.Join(errors.New("error while reading seek table header"), err)
	}
	if binary.LittleEndian.Uint32(header[0:4]) != headerMagicNumber {
		return nil, errors.Join(ErrInvalidSeekTable, ErrInvalidSeekTableHeaderMagicNumber)
	}
	frameSize := binary.LittleEndian.Uint32(header[4:8])
	if frameSize != (numEntries*8)+9 {
		return nil, errors.Join(ErrInvalidSeekTable, ErrSeekTableSizeMismatch)
	}

	// Read entries
	entriesData := make([]byte, numEntries*8)
	_, err = io.ReadFull(data, entriesData)
	if err != nil {
		return nil, errors.Join(errors.New("error while reading seek table entries"), err)
	}
	for i := uint32(0); i < numEntries; i += 4 { // Every value of the entry must be greater than zero. Empty chunks are not allowed.
		if entriesData[0] == 0 && entriesData[1] == 0 && entriesData[2] == 0 && entriesData[3] == 0 {
			return nil, errors.Join(ErrInvalidSeekTable, errors.New("seek table contains empty chunk entry"))
		}
	}

	return &Table{entries: entriesData}, nil
}

func WriteTableToWriter(t *Table, w io.Writer) (int64, error) {
	header := [8]byte{
		0x00, 0x00, 0x00, 0x00, // magic number in little endian
		0x00, 0x00, 0x00, 0x00, // frame size in little endian
	}
	binary.LittleEndian.PutUint32(header[0:4], 0x184D2A5E)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(t.entries)+9)) // +9 for the header size
	headerBytes, err := w.Write(header[:])
	if err != nil {
		return int64(headerBytes), errors.Join(errors.New("error while writing seek table header"), err)
	}

	entriesBytes, err := w.Write(t.entries)
	if err != nil {
		return int64(headerBytes + entriesBytes), errors.Join(errors.New("error while writing seek table entries"), err)
	}

	footer := [9]byte{
		0x00, 0x00, 0x00, 0x00, // number of entries in little endian
		0x00,                   // descriptor: 7 Checksum_Flag, 6-2 Reserved_Bits, 1-0	Unused_Bits
		0x00, 0x00, 0x00, 0x00, // magic number
	}
	binary.LittleEndian.PutUint32(footer[0:4], uint32(t.NumEntries()))
	binary.LittleEndian.PutUint32(footer[5:9], footerMagicNumber)
	footerBytes, err := w.Write(footer[:])
	if err != nil {
		return int64(headerBytes + entriesBytes + footerBytes), errors.Join(errors.New("error while writing seek table footer"), err)
	}

	return int64(headerBytes + entriesBytes + footerBytes), err
}
