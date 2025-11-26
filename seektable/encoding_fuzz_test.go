package seektable

import (
	"bytes"
	"testing"
)

func FuzzWriteRead(f *testing.F) {
	// Number of entries | seed for entries generator
	f.Add(0, 10)
	f.Add(1, 42)
	f.Add(1000, 99)

	f.Fuzz(func(t *testing.T, numEntries int, seed int) {
		// Skip negative or excessively large values
		numEntries = int(uint(numEntries) % 1001)

		// Create a table with random entries
		table := Table{}
		for i := 0; i < numEntries; i++ {
			// Generate pseudo-random but deterministic values based on seed and index
			// Use multiple mixing operations to fill entire uint32 range
			s := uint32(seed + i*1337)
			s ^= s << 13
			s ^= s >> 17
			s ^= s << 5
			compressedSize := s

			// Generate different random value for decompressed size
			d := uint32(seed*31 + i*97 + 0x9E3779B9)
			d ^= d << 11
			d ^= d >> 19
			d ^= d << 7
			decompressedSize := d

			table.AppendEntry(TableEntry{
				CompressedSize:   compressedSize,
				DecompressedSize: decompressedSize,
			})
		}

		// Write the table to a buffer
		var buf bytes.Buffer
		n, err := WriteTableToWriter(&table, &buf)
		if err != nil {
			t.Fatalf("WriteTableToWriter failed: %v", err)
		}

		// Expected size: 8 (header) + (numEntries * 8) + 9 (footer)
		expectedSize := int64(8 + numEntries*8 + 9)
		if n != expectedSize {
			t.Errorf("WriteTableToWriter wrote %d bytes, expected %d", n, expectedSize)
		}

		// Read the table back
		reader := bytes.NewReader(buf.Bytes())
		readTable, err := ReadTableFromReadSeeker(reader)
		if err != nil {
			t.Fatalf("ReadTableFromReadSeeker failed: %v", err)
		}

		// Verify the table has the same number of entries
		if readTable.NumEntries() != table.NumEntries() {
			t.Fatalf("NumEntries mismatch: got %d, expected %d", readTable.NumEntries(), table.NumEntries())
		}

		// Verify all entry values match
		for i := 0; i < table.NumEntries(); i++ {
			originalEntry := table.GetEntry(i)
			readEntry := readTable.GetEntry(i)
			if readEntry.CompressedSize != originalEntry.CompressedSize {
				t.Errorf("Entry %d CompressedSize mismatch: got %d, expected %d",
					i, readEntry.CompressedSize, originalEntry.CompressedSize)
			}
			if readEntry.DecompressedSize != originalEntry.DecompressedSize {
				t.Errorf("Entry %d DecompressedSize mismatch: got %d, expected %d",
					i, readEntry.DecompressedSize, originalEntry.DecompressedSize)
			}
		}
	})
}
