package seektable

import (
	"testing"
)

func FuzzTableFind(f *testing.F) {
	// Number of blocks | seed for block generator | offset to search for
	f.Add(0, 10, uint64(0))
	f.Add(1, 42, uint64(100))
	f.Add(10, 99, uint64(1000))
	f.Add(100, 123, uint64(50000))
	f.Add(1000, 456, uint64(1000000))

	f.Fuzz(func(t *testing.T, numBlocks int, seed int, searchOffset uint64) {
		// Limit number of blocks to reasonable range
		numBlocks = int(uint(numBlocks) % 1001)
		searchOffset = searchOffset % (uint64(4294967295)*uint64(numBlocks) + 1) // limit search offset to plausible range

		if numBlocks == 0 {
			// Empty table should always return false
			table := Table{}
			_, found := table.Find(searchOffset)
			if found {
				t.Errorf("Find on empty table should return false")
			}
			return
		}

		// Create a table with random block entries
		table := Table{}
		var totalDecompressedSize uint64
		for i := 0; i < numBlocks; i++ {
			// Generate pseudo-random but deterministic values
			s := uint32(seed + i*1337)
			s ^= s << 13
			s ^= s >> 17
			s ^= s << 5
			compressedSize := s

			d := uint32(seed*31 + i*97 + 0x9E3779B9)
			d ^= d << 11
			d ^= d >> 19
			d ^= d << 7
			decompressedSize := d

			table.AppendEntry(TableEntry{
				CompressedSize:   compressedSize,
				DecompressedSize: decompressedSize,
			})
			totalDecompressedSize += uint64(decompressedSize)
		}

		// Test Find function
		result, found := table.Find(searchOffset)

		if !found {
			// Offset should be beyond the table range
			if searchOffset < totalDecompressedSize {
				t.Errorf("Find returned false for offset %d, but total decompressed size is %d",
					searchOffset, totalDecompressedSize)
			}
			return
		}

		// Verify the result is valid
		if result.EntryIndex < 0 || result.EntryIndex >= numBlocks {
			t.Fatalf("Find returned invalid EntryIndex: %d (numBlocks: %d)",
				result.EntryIndex, numBlocks)
		}

		entry := table.GetEntry(result.EntryIndex)

		// Verify the offset is within the found entry's range
		entryStart := result.EntryOffsetInDecompressed
		entryEnd := entryStart + uint64(entry.DecompressedSize)
		if searchOffset < entryStart || searchOffset >= entryEnd {
			t.Errorf("Find returned entry %d with range [%d, %d), but search offset is %d",
				result.EntryIndex, entryStart, entryEnd, searchOffset)
		}

		// Verify EntryOffsetInCompressed and EntryOffsetInDecompressed are consistent
		var expectedCompressedOffset, expectedDecompressedOffset uint64
		for i := 0; i < result.EntryIndex; i++ {
			e := table.GetEntry(i)
			expectedCompressedOffset += uint64(e.CompressedSize)
			expectedDecompressedOffset += uint64(e.DecompressedSize)
		}

		if result.EntryOffsetInCompressed != expectedCompressedOffset {
			t.Errorf("EntryOffsetInCompressed mismatch: got %d, expected %d",
				result.EntryOffsetInCompressed, expectedCompressedOffset)
		}
		if result.EntryOffsetInDecompressed != expectedDecompressedOffset {
			t.Errorf("EntryOffsetInDecompressed mismatch: got %d, expected %d",
				result.EntryOffsetInDecompressed, expectedDecompressedOffset)
		}
	})
}
