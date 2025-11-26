package seektable

import (
	"encoding/binary"
	"sync"
)

type TableEntry struct {
	CompressedSize   uint32
	DecompressedSize uint32
}

type Table struct {
	entries []byte

	cached        sync.Once
	cachedOffsets []TableOffset
}

type TableOffset struct {
	EntryIndex                int
	EntryOffsetInCompressed   uint64
	EntryOffsetInDecompressed uint64
}

func (t *Table) GetEntry(index int) TableEntry {
	offset := index * 8
	return TableEntry{
		CompressedSize:   binary.LittleEndian.Uint32(t.entries[offset : offset+4]),
		DecompressedSize: binary.LittleEndian.Uint32(t.entries[offset+4 : offset+8]),
	}
}

func (t *Table) AppendEntry(entry TableEntry) {
	t.entries = append(t.entries, 0, 0, 0, 0, 0, 0, 0, 0)
	t.SetEntry(t.NumEntries()-1, entry)
}

func (t *Table) SetEntry(index int, entry TableEntry) {
	offset := index * 8
	binary.LittleEndian.PutUint32(t.entries[offset:offset+4], entry.CompressedSize)
	binary.LittleEndian.PutUint32(t.entries[offset+4:offset+8], entry.DecompressedSize)
}

func (t *Table) NumEntries() int {
	return len(t.entries) / 8
}

func (t *Table) OffsetsByIndex(index int) TableOffset {
	t.CacheOffsets()

	return t.cachedOffsets[index]
}

func (t *Table) Size() int {
	return len(t.entries) + 8 + 9 // entries + header + footer
}

// Get the TableOffset for a given decompressed offset
func (t *Table) Find(offset uint64) (TableOffset, bool) {
	t.CacheOffsets()

	// Search linearly for small tables
	if len(t.cachedOffsets) < 32 {
		for _, to := range t.cachedOffsets {
			if to.EntryOffsetInDecompressed <= offset && offset < to.EntryOffsetInDecompressed+uint64(t.GetEntry(to.EntryIndex).DecompressedSize) {
				return to, true
			}
		}
		return TableOffset{}, false
	}

	// Binary search for larger tables
	low, high := 0, len(t.cachedOffsets)-1
	for low <= high {
		mid := (low + high) / 2
		to := t.cachedOffsets[mid]
		entry := t.GetEntry(to.EntryIndex)
		if to.EntryOffsetInDecompressed <= offset && offset < to.EntryOffsetInDecompressed+uint64(entry.DecompressedSize) {
			return to, true
		} else if offset < to.EntryOffsetInDecompressed {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}
	return TableOffset{}, false
}

// CacheOffsets precomputes the decompressed and compressed offsets for each entry.
// Can be run multiple times safely. Will be run automatically on first Find call if not run before.
func (t *Table) CacheOffsets() {
	t.cached.Do(func() {
		offsets := make([]TableOffset, t.NumEntries())
		var compressedOffset, decompressedOffset uint64
		for i := 0; i < t.NumEntries(); i++ {
			entry := t.GetEntry(i)
			offsets[i] = TableOffset{
				EntryIndex:                i,
				EntryOffsetInCompressed:   compressedOffset,
				EntryOffsetInDecompressed: decompressedOffset,
			}
			compressedOffset += uint64(entry.CompressedSize)
			decompressedOffset += uint64(entry.DecompressedSize)
		}
		t.cachedOffsets = offsets
	})
}
