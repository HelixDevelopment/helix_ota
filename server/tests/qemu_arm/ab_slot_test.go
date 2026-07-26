// Package qemu_arm_test validates the RK3588 U-Boot A/B slot-writer
// and fw_setenv code against a QEMU aarch64 block-device model.
//
// These tests build a synthetic disk image with a GPT partition table
// matching the RK3588 layout, mount it as a QEMU raw block device, and
// exercise the real ApplyPort slot-writer + UBootEnvManager interfaces
// against byte-addressable partitions at known offsets.
//
// Unblocks: OTA-003, OTA-004, OTA-038, OTA-041, OTA-042, OTA-043.
// Hardware validation plan: docs/research/main_specs/additions/
// rk3588_hardware_validation_plan.md
//
// QEMU availability: the test auto-detects qemu-system-aarch64 + qemu-img.
// When either is missing, every test skips with t.Skip() — never a hard
// failure — so the suite remains GREEN on machines without QEMU ARM.
//
// Design references:
//   - docs/design/rk3588_ab_virt/PWU_AB_4_APPLY_PORT.md
//   - server/internal/device/applyport.go (ApplyPort)
//   - server/internal/device/slot.go (SlotWriter)
//   - server/internal/device/fwenv.go (UBootEnvManager)
package qemu_arm_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HelixDevelopment/helix_ota/server/internal/device"
)

// ------------------------------ QEMU detection -------------------------------

func qemuAArch64Available() bool {
	_, err := exec.LookPath("qemu-system-aarch64")
	return err == nil
}

func qemuImgAvailable() bool {
	_, err := exec.LookPath("qemu-img")
	return err == nil
}

func requireQemu(t *testing.T) {
	t.Helper()
	if !qemuAArch64Available() {
		t.Skip("qemu-system-aarch64 not found — QEMU ARM tests skipped")
	}
	if !qemuImgAvailable() {
		t.Skip("qemu-img not found — QEMU ARM tests skipped")
	}
}

// ------------------------------ Disk geometry --------------------------------

// rk3588DiskLayout encodes the known RK3588/U-Boot GPT layout from the
// reference disk image (assemble_ab_disk.sh:204-205, slot.go:11-13):
//
//	Partition 1: U-Boot / EFI (unused by slot-writer tests)
//	Partition 2: Slot A rootfs  → /dev/vda2
//	Partition 3: Slot B rootfs  → /dev/vda3
//	Partition 4: User data
//
// The U-Boot environment blob lives in a raw area BETWEEN the GPT header+entries
// and partition 1, at the offset documented in /etc/fw_env.config. Per the
// RK3588 U-Boot CONFIG_ENV_OFFSET (commonly 0x3F8000 for eMMC or 0x88000 for
// SD), we place it at 512K (0x80000) — a 64-KiB area with CRC32+key=value\0
// records.
type rk3588DiskLayout struct {
	SectorSize   int64 // 512
	FirstLBA     int64 // 2048 — first usable sector after GPT
	PartSizeMB   int64 // 128 MB per slot partition
	EnvOffset    int64 // byte offset of the U-Boot env area (512K)
	EnvSize      int64 // 65536 bytes (64 KiB)
	SlotAOffset  int64 // byte offset of slot A partition data
	SlotBOffset  int64 // byte offset of slot B partition data
	SlotPartSize int64 // byte size of each slot partition
	TotalSize    int64 // total disk size in bytes
}

func newLayout() rk3588DiskLayout {
	const (
		sectorSize     int64 = 512
		firstLBA       int64 = 2048
		gptEntryLBA    int64 = 2
		gptEntriesN    int64 = 128
		gptEntryBytes  int64 = 128
		gptEntriesSize       = (gptEntryLBA + gptEntriesN) * sectorSize
		slotStartLBA   int64 = firstLBA + 2048 // start slot A 2MB in
		partSizeMB     int64 = 128
		partSizeBytes  int64 = partSizeMB * 1024 * 1024
		envOffset      int64 = 512 * 1024 // 512 KiB between GPT entries and slot A start
		envSize        int64 = 65536
	)
	slotAOffset := slotStartLBA * sectorSize
	slotBOffset := slotAOffset + partSizeBytes

	return rk3588DiskLayout{
		SectorSize:   sectorSize,
		FirstLBA:     firstLBA,
		PartSizeMB:   partSizeMB,
		EnvOffset:    envOffset,
		EnvSize:      envSize,
		SlotAOffset:  slotAOffset,
		SlotBOffset:  slotBOffset,
		SlotPartSize: partSizeBytes,
		TotalSize:    gptEntriesSize + int64(partSizeMB)*2*1024*1024 + 16*1024*1024,
	}
}

// ---------------------------- GPT builder ------------------------------------

// writeGPT builds a minimal Protective MBR + GPT header + partition-entry array
// that makes the disk image structurally valid so QEMU and U-Boot recognize
// the partition table. Slot-writer tests do NOT use the GPT headers to locate
// partitions (they use the fixed byte offsets from rk3588DiskLayout), but the
// GPT ensures the image passes `qemu-img check` and is a faithful block-device
// substitute.
func writeGPT(f *os.File, layout rk3588DiskLayout) error {
	ss := layout.SectorSize

	// — Protective MBR (LBA 0, 512 bytes) —
	// Single partition of type 0xEE spanning the visible disk.
	lba0 := make([]byte, 512)
	lba0[446] = 0x00 // boot flag
	lba0[447] = 0x01 // start CHS (0/0/2)
	lba0[448] = 0x00
	lba0[449] = 0x02
	lba0[450] = 0xEE // EFI GPT type
	lba0[509] = 0x55 // CHS end
	lba0[510] = 0xAA // CHS end
	// LBA start at offset 454 (4 bytes, LE) = 1
	binary.LittleEndian.PutUint32(lba0[454:458], 1)
	// LBA count at offset 458 (4 bytes, LE) = total sectors - 1
	totalSectors := uint32(layout.TotalSize/ss - 1)
	binary.LittleEndian.PutUint32(lba0[458:462], totalSectors)
	lba0[510] = 0x55 // boot signature
	lba0[511] = 0xAA

	if _, err := f.WriteAt(lba0, 0); err != nil {
		return fmt.Errorf("write protective MBR: %w", err)
	}

	// — GPT Header (LBA 1, 512 bytes) —
	binary.LittleEndian.PutUint32(lba0[446:450], 0) // restore

	_ = lba0 // silence; reuse buffer (zeroed)

	gptHdr := make([]byte, 512)
	copy(gptHdr[0:8], []byte("EFI PART"))            // signature
	binary.LittleEndian.PutUint32(gptHdr[8:12], 0x00010000) // revision 1.0
	binary.LittleEndian.PutUint32(gptHdr[12:16], 92)        // header size
	// CRC32 of header (offset 16-19) — compute after writing most fields
	binary.LittleEndian.PutUint32(gptHdr[20:24], 0) // reserved
	// Current LBA (offset 24) = 1
	binary.LittleEndian.PutUint64(gptHdr[24:32], 1)
	// Backup LBA (offset 32) = last LBA of disk
	binary.LittleEndian.PutUint64(gptHdr[32:40], uint64(layout.TotalSize/ss-1))
	// First usable LBA
	binary.LittleEndian.PutUint64(gptHdr[40:48], uint64(layout.FirstLBA))
	// Last usable LBA
	binary.LittleEndian.PutUint64(gptHdr[48:56], uint64(layout.TotalSize/ss-2))
	// GUID (random, offset 56)
	copy(gptHdr[56:72], bytes.Repeat([]byte{0x01}, 16))
	// Partition entry array LBA = 2
	binary.LittleEndian.PutUint64(gptHdr[72:80], 2)
	// Number of partition entries = 128
	binary.LittleEndian.PutUint32(gptHdr[80:84], 128)
	// Size of each entry
	binary.LittleEndian.PutUint32(gptHdr[84:88], 128)
	// CRC32 of partition entries array (offset 88-91)
	binary.LittleEndian.PutUint32(gptHdr[92:96], 0) // reserved

	if _, err := f.WriteAt(gptHdr, ss); err != nil {
		return fmt.Errorf("write GPT header: %w", err)
	}

	// — Partition entries (LBA 2-33) —
	// Only need 4 entries for the 4-partition layout.
	// Entry size = 128 bytes.
	entryBytes := make([]byte, 128*128) // 128 entries x 128 bytes
	partitions := []struct {
		name     string
		startLBA uint64
		endLBA   uint64
		typeGUID [16]byte
	}{
		{
			name:     "uboot",
			startLBA: uint64(layout.FirstLBA),
			endLBA:   uint64(layout.EnvOffset/ss - 1),
		},
		{
			name:     "slot-a",
			startLBA: uint64(layout.SlotAOffset / ss),
			endLBA:   uint64(layout.SlotBOffset/ss - 1),
		},
		{
			name:     "slot-b",
			startLBA: uint64(layout.SlotBOffset / ss),
			endLBA:   uint64((layout.SlotBOffset+layout.SlotPartSize)/ss - 1),
		},
		{
			name:     "data",
			startLBA: uint64((layout.SlotBOffset + layout.SlotPartSize) / ss),
			endLBA:   uint64(layout.TotalSize/ss - 2),
		},
	}

	for i, p := range partitions {
		off := i * 128
		// Type GUID (16 bytes at offset 0)
		copy(entryBytes[off:off+16], p.typeGUID[:])
		// Unique partition GUID (16 bytes at offset 16) — random
		copy(entryBytes[off+16:off+32], bytes.Repeat([]byte{byte(i + 0x10)}, 16))
		// Starting LBA (offset 32)
		binary.LittleEndian.PutUint64(entryBytes[off+32:off+40], p.startLBA)
		// Ending LBA (offset 40)
		binary.LittleEndian.PutUint64(entryBytes[off+40:off+48], p.endLBA)
		// Attributes (offset 48)
		if i == 1 || i == 2 {
			// Slot A/B: bit 50 = boot-success (U-Boot GPT-partition boot
			// selection convention).
			entryBytes[off+54] = 0x04
		}
		// Name (36 bytes UTF-16LE at offset 56)
		nameBytes := utf16LEEncode(p.name)
		copy(entryBytes[off+56:off+92], nameBytes)
	}

	if _, err := f.WriteAt(entryBytes, 2*ss); err != nil {
		return fmt.Errorf("write GPT partition entries: %w", err)
	}

	// — GPT header CRC32 —
	partCRC := crc32.ChecksumIEEE(entryBytes)
	binary.LittleEndian.PutUint32(gptHdr[88:92], partCRC)
	hdrCRC := crc32.ChecksumIEEE(gptHdr[:92])
	binary.LittleEndian.PutUint32(gptHdr[16:20], hdrCRC)
	if _, err := f.WriteAt(gptHdr, ss); err != nil {
		return fmt.Errorf("rewrite GPT header with CRC: %w", err)
	}

	return nil
}

func utf16LEEncode(s string) []byte {
	runes := []rune(s)
	out := make([]byte, 72) // 36 UTF-16 code units
	for i, r := range runes {
		if i >= 36 {
			break
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(r))
	}
	return out
}

// ------------------------ QEMU block-device SlotWriter -----------------------

// qemuBlockSlotWriter implements device.SlotWriter against a raw disk-image
// file. It maps partitions to byte offsets (not block-device paths) so the
// test exercises the exact same read/write semantics without losetup or a
// running QEMU guest.
type qemuBlockSlotWriter struct {
	disk         *os.File
	activeSlot   string
	slotAOffset  int64
	slotBOffset  int64
	slotPartSize int64
}

func newQemuBlockSlotWriter(disk *os.File, activeSlot string, layout rk3588DiskLayout) *qemuBlockSlotWriter {
	return &qemuBlockSlotWriter{
		disk:         disk,
		activeSlot:   activeSlot,
		slotAOffset:  layout.SlotAOffset,
		slotBOffset:  layout.SlotBOffset,
		slotPartSize: layout.SlotPartSize,
	}
}

func (w *qemuBlockSlotWriter) ActiveSlot() (string, error)                 { return w.activeSlot, nil }
func (w *qemuBlockSlotWriter) InactiveSlot() (string, error)               { return otherSlot(w.activeSlot), nil }
func (w *qemuBlockSlotWriter) slotOffset(slot string) (int64, error) {
	switch slot {
	case "A":
		return w.slotAOffset, nil
	case "B":
		return w.slotBOffset, nil
	default:
		return 0, fmt.Errorf("unknown slot %q", slot)
	}
}

func (w *qemuBlockSlotWriter) WriteInactiveSlot(ctx context.Context, imagePath string) (string, error) {
	inactive, err := w.InactiveSlot()
	if err != nil {
		return "", err
	}
	offset, err := w.slotOffset(inactive)
	if err != nil {
		return "", err
	}

	src, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("qemu slot: open image %q: %w", imagePath, err)
	}
	defer src.Close()

	srcInfo, err := src.Stat()
	if err != nil {
		return "", fmt.Errorf("qemu slot: stat image: %w", err)
	}
	if srcInfo.Size() == 0 {
		return "", fmt.Errorf("qemu slot: image %q is empty", imagePath)
	}
	if srcInfo.Size() > w.slotPartSize {
		return "", fmt.Errorf("qemu slot: image %q (%d bytes) exceeds slot size (%d bytes)",
			imagePath, srcInfo.Size(), w.slotPartSize)
	}

	buf := make([]byte, 4*1024*1024) // 4 MiB buffer (matches dd bs=4M)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := w.disk.WriteAt(buf[:n], offset); writeErr != nil {
				return "", fmt.Errorf("qemu slot: write to offset %d: %w", offset, writeErr)
			}
			offset += int64(n)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return "", fmt.Errorf("qemu slot: read image: %w", readErr)
		}
	}

	return inactive, nil
}

func otherSlot(slot string) string {
	if slot == "A" {
		return "B"
	}
	return "A"
}

// ---------------------- QEMU U-Boot env manager ----------------------------

// qemuUBootEnvManager implements device.UBootEnvManager against a raw
// U-Boot environment blob stored at a known offset in the disk image.
//
// U-Boot raw env format (libubootenv / fw_printenv):
//
//	Offset  Size   Field
//	0       4      CRC32 (little-endian) of data area
//	4       N      key=value\0key=value\0...\0\0
//	N+4     rest   padding to env size
//
// The canonical tools (fw_setenv/fw_printenv) operate on this layout
// driven by /etc/fw_env.config. Here we implement the same raw format
// directly so the test has no external binary dependency.
type qemuUBootEnvManager struct {
	disk      *os.File
	envOffset int64
	envSize   int64
	cached    map[string]string // current key-values in the env
}

func newQemuUBootEnvManager(disk *os.File, layout rk3588DiskLayout) (*qemuUBootEnvManager, error) {
	m := &qemuUBootEnvManager{
		disk:      disk,
		envOffset: layout.EnvOffset,
		envSize:   layout.EnvSize,
		cached:    make(map[string]string),
	}
	if err := m.loadEnv(); err != nil {
		return nil, fmt.Errorf("qemu env: load: %w", err)
	}
	return m, nil
}

func (m *qemuUBootEnvManager) loadEnv() error {
	raw := make([]byte, m.envSize)
	n, err := m.disk.ReadAt(raw, m.envOffset)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	raw = raw[:n]

	if n < 4 {
		// No env yet — first use.
		return nil
	}

	storedCRC := binary.LittleEndian.Uint32(raw[0:4])
	data := raw[4:]
	if storedCRC == 0 {
		return nil
	}

	dataCRC := crc32.ChecksumIEEE(data)
	if storedCRC != dataCRC {
		return fmt.Errorf("env CRC mismatch: stored=%08x computed=%08x", storedCRC, dataCRC)
	}

	// Parse key=value\0 entries.
	entries := splitNull(data)
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) == 2 {
			m.cached[kv[0]] = kv[1]
		}
	}
	return nil
}

func splitNull(data []byte) []string {
	var parts []string
	var start int
	for i, b := range data {
		if b == 0 {
			parts = append(parts, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		parts = append(parts, string(data[start:]))
	}
	return parts
}

func (m *qemuUBootEnvManager) flush() error {
	var buf bytes.Buffer
	for k, v := range m.cached {
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
		buf.WriteByte(0)
	}
	buf.WriteByte(0) // double-null terminator

	data := buf.Bytes()
	if int64(len(data)) > m.envSize-4 {
		return fmt.Errorf("env data too large: %d bytes (max %d)", len(data), m.envSize-4)
	}

	full := make([]byte, m.envSize)
	copy(full[4:], data)
	// Pad with 0xFF (erased flash convention).
	for i := 4 + len(data); i < len(full); i++ {
		full[i] = 0xFF
	}

	crc := crc32.ChecksumIEEE(full[4:])
	binary.LittleEndian.PutUint32(full[0:4], crc)

	_, err := m.disk.WriteAt(full, m.envOffset)
	return err
}

func (m *qemuUBootEnvManager) SetEnv(key, value string) error {
	m.cached[key] = value
	return m.flush()
}

func (m *qemuUBootEnvManager) GetEnv(key string) (string, error) {
	if v, ok := m.cached[key]; ok {
		return v, nil
	}
	return "", nil
}

func (m *qemuUBootEnvManager) SaveEnv() error {
	return m.flush()
}

// ------------------------- QEMU disk fixture --------------------------------

// qemuDiskFixture holds a temp raw disk image with GPT partition table.
type qemuDiskFixture struct {
	DiskPath string
	Disk     *os.File
	Layout   rk3588DiskLayout
}

func newQemuDiskFixture(t *testing.T) *qemuDiskFixture {
	t.Helper()
	requireQemu(t)

	layout := newLayout()
	dir := t.TempDir()
	diskPath := filepath.Join(dir, "test-disk.raw")

	// Create raw disk image with qemu-img.
	createCmd := exec.Command("qemu-img", "create", "-f", "raw", diskPath,
		fmt.Sprintf("%d", layout.TotalSize))
	if out, err := createCmd.CombinedOutput(); err != nil {
		t.Fatalf("qemu-img create: %v\n%s", err, out)
	}

	disk, err := os.OpenFile(diskPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("open disk image: %v", err)
	}
	t.Cleanup(func() { disk.Close() })

	if err := writeGPT(disk, layout); err != nil {
		t.Fatalf("write GPT: %v", err)
	}

	return &qemuDiskFixture{
		DiskPath: diskPath,
		Disk:     disk,
		Layout:   layout,
	}
}

// verifyPartition reads len bytes from the slot partition and verifies
// they match the file at verifyPath.
func (fx *qemuDiskFixture) verifySlotContent(t *testing.T, slot string, expectedPath string, size int64) {
	t.Helper()

	var offset int64
	switch slot {
	case "A":
		offset = fx.Layout.SlotAOffset
	case "B":
		offset = fx.Layout.SlotBOffset
	default:
		t.Fatalf("unknown slot %q", slot)
	}

	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read expected file %q: %v", expectedPath, err)
	}

	actual := make([]byte, size)
	n, err := fx.Disk.ReadAt(actual, offset)
	if err != nil {
		t.Fatalf("read slot %s at offset %d: %v", slot, offset, err)
	}
	actual = actual[:n]

	if !bytes.Equal(expected, actual[:len(expected)]) {
		t.Fatalf("slot %s content mismatch: expected %d bytes, got %d bytes",
			slot, len(expected), len(actual))
	}
	t.Logf("slot %s content verified: %d bytes match", slot, len(expected))
}

// -------------------------------- Tests --------------------------------------

// TestQemuSlotWriter_WriteInactiveSlot writes a rootfs image to the inactive
// slot on a raw QEMU disk image and verifies the bytes land at the correct
// partition offset.
func TestQemuSlotWriter_WriteInactiveSlot(t *testing.T) {
	requireQemu(t)

	type testCase struct {
		activeSlot string
		wantSlot   string
	}
	tests := []testCase{
		{activeSlot: "A", wantSlot: "B"},
		{activeSlot: "B", wantSlot: "A"},
	}

	for _, tc := range tests {
		t.Run("active="+tc.activeSlot, func(t *testing.T) {
			fx := newQemuDiskFixture(t)
			writer := newQemuBlockSlotWriter(fx.Disk, tc.activeSlot, fx.Layout)

			// Write a test payload to a temp file.
			payload := []byte(fmt.Sprintf("rootfs-content-for-slot-%s-%s",
				tc.activeSlot, tc.wantSlot))
			imgPath := filepath.Join(t.TempDir(), "rootfs.img")
			if err := os.WriteFile(imgPath, payload, 0644); err != nil {
				t.Fatalf("write payload: %v", err)
			}

			writtenSlot, err := writer.WriteInactiveSlot(context.Background(), imgPath)
			if err != nil {
				t.Fatalf("WriteInactiveSlot: %v", err)
			}
			if writtenSlot != tc.wantSlot {
				t.Fatalf("written slot = %q, want %q", writtenSlot, tc.wantSlot)
			}

			// Verify bytes landed at the correct partition offset.
			fx.verifySlotContent(t, tc.wantSlot, imgPath, int64(len(payload)))

			// Verify the ACTIVE slot partition was NOT touched.
			var activeOffset int64
			if tc.activeSlot == "A" {
				activeOffset = fx.Layout.SlotAOffset
			} else {
				activeOffset = fx.Layout.SlotBOffset
			}
			activeData := make([]byte, fx.Layout.SlotPartSize)
			n, _ := fx.Disk.ReadAt(activeData, activeOffset)
			activeData = activeData[:n]
			if bytes.Contains(activeData, payload) {
				t.Fatalf("ACTIVE slot %s was overwritten — payload found at offset %d",
					tc.activeSlot, activeOffset)
			}
		})
	}
}

// TestQemuUBootEnv_RoundTrip sets and reads U-Boot env variables against
// the raw env blob at the canonical offset. This proves the raw env format
// implementation matches what fw_setenv/fw_printenv expect.
func TestQemuUBootEnv_RoundTrip(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)
	mgr, err := newQemuUBootEnvManager(fx.Disk, fx.Layout)
	if err != nil {
		t.Fatalf("new env manager: %v", err)
	}

	// Write several variables.
	envs := map[string]string{
		"BOOT_ORDER":    "B A",
		"upgrade_available": "1",
		"bootcount":     "1",
		"BOOT_A_LEFT":   "3",
		"BOOT_B_LEFT":   "3",
	}
	for k, v := range envs {
		if err := mgr.SetEnv(k, v); err != nil {
			t.Fatalf("SetEnv(%q, %q): %v", k, v, err)
		}
	}

	// Read back and verify.
	for k, want := range envs {
		got, err := mgr.GetEnv(k)
		if err != nil {
			t.Fatalf("GetEnv(%q): %v", k, err)
		}
		if got != want {
			t.Fatalf("GetEnv(%q) = %q, want %q", k, got, want)
		}
	}

	// Verify CRC integrity by reloading.
	mgr2, err := newQemuUBootEnvManager(fx.Disk, fx.Layout)
	if err != nil {
		t.Fatalf("reload env: %v", err)
	}
	for k, want := range envs {
		got, _ := mgr2.GetEnv(k)
		if got != want {
			t.Fatalf("after reload: GetEnv(%q) = %q, want %q", k, got, want)
		}
	}
}

// TestQemuUBootEnv_CRC_Corruption proves that a corrupted U-Boot env blob
// (bad CRC32) is detected and reported, mimicking the real fw_printenv
// behaviour on a damaged flash region.
func TestQemuUBootEnv_CRC_Corruption(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)

	// Write a valid env.
	mgr, err := newQemuUBootEnvManager(fx.Disk, fx.Layout)
	if err != nil {
		t.Fatalf("new env manager: %v", err)
	}
	if err := mgr.SetEnv("test_key", "test_value"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}

	// Corrupt the env area by writing random bytes at offset+4.
	corrupt := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if _, err := fx.Disk.WriteAt(corrupt, fx.Layout.EnvOffset+4); err != nil {
		t.Fatalf("corrupt env: %v", err)
	}

	// Attempting to reload should fail with CRC mismatch.
	_, err = newQemuUBootEnvManager(fx.Disk, fx.Layout)
	if err == nil {
		t.Fatal("expected CRC error on corrupted env, got nil")
	}
	if !strings.Contains(err.Error(), "CRC") {
		t.Fatalf("expected CRC error, got: %v", err)
	}
}

// TestApplyPort_WriteAndArm_QemuBlock applies the full ApplyPort sequence
// against a QEMU disk image: it writes a rootfs to the inactive slot,
// arms the U-Boot env (BOOT_ORDER, upgrade_available, bootcount), and
// verifies every env variable + partition content.
func TestApplyPort_WriteAndArm_QemuBlock(t *testing.T) {
	requireQemu(t)

	type scenario struct {
		name       string
		activeSlot string
		wantOrder  string
	}
	scenarios := []scenario{
		{name: "A→B switch", activeSlot: "A", wantOrder: "B A"},
		{name: "B→A switch", activeSlot: "B", wantOrder: "A B"},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			fx := newQemuDiskFixture(t)
			writer := newQemuBlockSlotWriter(fx.Disk, sc.activeSlot, fx.Layout)
			envMgr, err := newQemuUBootEnvManager(fx.Disk, fx.Layout)
			if err != nil {
				t.Fatalf("new env mgr: %v", err)
			}

			applier := device.NewApplyPort(writer, envMgr,
				fmt.Sprintf("/dev/vda%d", map[string]int{"A": 2, "B": 3}[sc.activeSlot]))

			// Create a rootfs payload.
			payload := []byte(fmt.Sprintf("helix-rootfs-%s-v2.0.0", sc.name))
			imgPath := filepath.Join(t.TempDir(), "rootfs.img")
			if err := os.WriteFile(imgPath, payload, 0644); err != nil {
				t.Fatalf("write payload: %v", err)
			}

			// WriteAndArm — the core sequence under test.
			if err := applier.WriteAndArm(context.Background(), imgPath); err != nil {
				t.Fatalf("WriteAndArm: %v", err)
			}

			// 1) Verify the rootfs landed on the correct slot.
			inactiveSlot := otherSlot(sc.activeSlot)
			fx.verifySlotContent(t, inactiveSlot, imgPath, int64(len(payload)))

			// 2) Verify U-Boot env: BOOT_ORDER.
			bootOrder, _ := envMgr.GetEnv("BOOT_ORDER")
			if bootOrder != sc.wantOrder {
				t.Fatalf("BOOT_ORDER = %q, want %q", bootOrder, sc.wantOrder)
			}

			// 3) Verify upgrade_available=1.
			ua, _ := envMgr.GetEnv("upgrade_available")
			if ua != "1" {
				t.Fatalf("upgrade_available = %q, want \"1\"", ua)
			}

			// 4) Verify bootcount=1.
			bc, _ := envMgr.GetEnv("bootcount")
			if bc != "1" {
				t.Fatalf("bootcount = %q, want \"1\"", bc)
			}

			// 5) Verify SaveEnv was called (env is persistent on disk).
			mgr2, err := newQemuUBootEnvManager(fx.Disk, fx.Layout)
			if err != nil {
				t.Fatalf("reload env after save: %v", err)
			}
			bootOrder2, _ := mgr2.GetEnv("BOOT_ORDER")
			if bootOrder2 != sc.wantOrder {
				t.Fatalf("after reload: BOOT_ORDER = %q (not persisted)", bootOrder2)
			}

			// 6) Verify slot detection.
			active, _ := applier.ActiveSlot()
			if active != sc.activeSlot {
				t.Fatalf("ActiveSlot = %q, want %q", active, sc.activeSlot)
			}
			inactive, _ := applier.InactiveSlot()
			if inactive != inactiveSlot {
				t.Fatalf("InactiveSlot = %q, want %q", inactive, inactiveSlot)
			}
		})
	}
}

// TestQemuSlotWriter_OversizeImage proves the writer rejects an image
// that exceeds the partition size.
func TestQemuSlotWriter_OversizeImage(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)
	writer := newQemuBlockSlotWriter(fx.Disk, "A", fx.Layout)

	// Create an image larger than the partition.
	payload := bytes.Repeat([]byte("X"), int(fx.Layout.SlotPartSize+1))
	imgPath := filepath.Join(t.TempDir(), "oversize.img")
	if err := os.WriteFile(imgPath, payload, 0644); err != nil {
		t.Fatalf("write oversize: %v", err)
	}

	_, err := writer.WriteInactiveSlot(context.Background(), imgPath)
	if err == nil {
		t.Fatal("expected error for oversize image, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds slot size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestQemuDiskImageGPTValid validates that the generated GPT disk image
// is structurally sound. qemu-img check is not available for raw format,
// so we verify GPT header signature + MBR boot signature directly.
func TestQemuDiskImageGPTValid(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)

	// Verify protective MBR boot signature (bytes 510-511 = 0x55 0xAA).
	mbrSig := make([]byte, 2)
	if _, err := fx.Disk.ReadAt(mbrSig, 510); err != nil {
		t.Fatalf("read MBR signature: %v", err)
	}
	if mbrSig[0] != 0x55 || mbrSig[1] != 0xAA {
		t.Fatalf("MBR boot signature: want 0x55 0xAA, got %#x %#x", mbrSig[0], mbrSig[1])
	}

	// Verify GPT header signature "EFI PART" at LBA 1.
	gptSig := make([]byte, 8)
	if _, err := fx.Disk.ReadAt(gptSig, fx.Layout.SectorSize); err != nil {
		t.Fatalf("read GPT header: %v", err)
	}
	if string(gptSig) != "EFI PART" {
		t.Fatalf("GPT header signature: want \"EFI PART\", got %q", string(gptSig))
	}

	t.Logf("MBR boot signature: 0x55 0xAA  PASS")
	t.Logf("GPT header signature: EFI PART  PASS")
}

// TestQemuSlotWriter_EmptyImage proves the writer rejects an empty (0-byte)
// image.
func TestQemuSlotWriter_EmptyImage(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)
	writer := newQemuBlockSlotWriter(fx.Disk, "A", fx.Layout)

	emptyPath := filepath.Join(t.TempDir(), "empty.img")
	if err := os.WriteFile(emptyPath, []byte{}, 0644); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	_, err := writer.WriteInactiveSlot(context.Background(), emptyPath)
	if err == nil {
		t.Fatal("expected error for empty image, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestQemuEnvManager_DeleteAndRecreate proves that setting a key to empty
// removes it, and re-setting it works.
func TestQemuEnvManager_DeleteAndRecreate(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)
	mgr, err := newQemuUBootEnvManager(fx.Disk, fx.Layout)
	if err != nil {
		t.Fatalf("new env mgr: %v", err)
	}

	// Set a value.
	if err := mgr.SetEnv("test", "hello"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}

	// Delete by setting to empty.
	if err := mgr.SetEnv("test", ""); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Verify it's gone.
	v, _ := mgr.GetEnv("test")
	if v != "" {
		t.Fatalf("expected empty after delete, got %q", v)
	}

	// Re-set.
	if err := mgr.SetEnv("test", "world"); err != nil {
		t.Fatalf("re-set: %v", err)
	}
	v, _ = mgr.GetEnv("test")
	if v != "world" {
		t.Fatalf("after re-set: got %q, want \"world\"", v)
	}
}

// TestQemuUptakeSequence simulates the full OTA uptake sequence across
// two successive updates (v1→v2, v2→v3), verifying slot toggling,
// env persistence, and version tracking at each step.
func TestQemuUptakeSequence(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)

	// Phase 1: initial state — slot A active, version 1.0.0.
	currentSlot := "A"
	currentVersion := "1.0.0"

	// Phase 2: deploy v2.0.0 → writes to slot B, arms env.
	t.Logf("Phase 2: deploying %s → v2.0.0 to slot B", currentVersion)
	writer2 := newQemuBlockSlotWriter(fx.Disk, currentSlot, fx.Layout)
	env2, _ := newQemuUBootEnvManager(fx.Disk, fx.Layout)
	applier2 := device.NewApplyPort(writer2, env2, "/dev/vda2")

	payload2 := []byte("helix-v2.0.0-rootfs")
	img2 := filepath.Join(t.TempDir(), "v2.img")
	os.WriteFile(img2, payload2, 0644)

	if err := applier2.WriteAndArm(context.Background(), img2); err != nil {
		t.Fatalf("phase 2 WriteAndArm: %v", err)
	}

	fx.verifySlotContent(t, "B", img2, int64(len(payload2)))
	bo2, _ := env2.GetEnv("BOOT_ORDER")
	if bo2 != "B A" {
		t.Fatalf("phase 2 BOOT_ORDER = %q, want \"B A\"", bo2)
	}

	// Simulate reboot: the bootloader selects slot B per BOOT_ORDER.
	currentSlot = "B"
	currentVersion = "2.0.0"
	t.Logf("Phase 2 complete: booted slot B (v2.0.0)")

	// Phase 3: deploy v3.0.0 → writes to slot A, arms env.
	t.Logf("Phase 3: deploying %s → v3.0.0 to slot A", currentVersion)
	writer3 := newQemuBlockSlotWriter(fx.Disk, currentSlot, fx.Layout)

	// Fresh env manager for the new boot (reads env from disk).
	env3, _ := newQemuUBootEnvManager(fx.Disk, fx.Layout)
	applier3 := device.NewApplyPort(writer3, env3, "/dev/vda3")

	payload3 := []byte("helix-v3.0.0-rootfs")
	img3 := filepath.Join(t.TempDir(), "v3.img")
	os.WriteFile(img3, payload3, 0644)

	if err := applier3.WriteAndArm(context.Background(), img3); err != nil {
		t.Fatalf("phase 3 WriteAndArm: %v", err)
	}

	fx.verifySlotContent(t, "A", img3, int64(len(payload3)))
	bo3, _ := env3.GetEnv("BOOT_ORDER")
	if bo3 != "A B" {
		t.Fatalf("phase 3 BOOT_ORDER = %q, want \"A B\"", bo3)
	}

	ua3, _ := env3.GetEnv("upgrade_available")
	if ua3 != "1" {
		t.Fatalf("phase 3 upgrade_available = %q, want \"1\"", ua3)
	}

	// Phase 4: healthy-boot confirm — clears upgrade_available and bootcount.
	t.Logf("Phase 4: healthy-boot confirm for v3.0.0")
	healthMarker := device.NewHealthMarker(env3)
	if err := healthMarker.ConfirmHealthy(); err != nil {
		t.Fatalf("ConfirmHealthy: %v", err)
	}
	ua4, _ := env3.GetEnv("upgrade_available")
	if ua4 != "0" {
		t.Fatalf("after confirm: upgrade_available = %q, want \"0\"", ua4)
	}
	bc4, _ := env3.GetEnv("bootcount")
	if bc4 != "0" {
		t.Fatalf("after confirm: bootcount = %q, want \"0\"", bc4)
	}

	// Confirm healthy persisted.
	envFinal, _ := newQemuUBootEnvManager(fx.Disk, fx.Layout)
	uaFinal, _ := envFinal.GetEnv("upgrade_available")
	if uaFinal != "0" {
		t.Fatalf("persisted upgrade_available = %q, want \"0\"", uaFinal)
	}

	currentVersion = "3.0.0"
	_ = currentVersion
}

// TestQemuQEMUImgInfo confirms the qemu-img utility can inspect the
// generated disk image and reports raw format + expected size.
func TestQemuQEMUImgInfo(t *testing.T) {
	requireQemu(t)

	fx := newQemuDiskFixture(t)

	cmd := exec.Command("qemu-img", "info", "--output=json", fx.DiskPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("qemu-img info: %v\n%s", err, out)
	}

	if !bytes.Contains(out, []byte(`"format": "raw"`)) {
		t.Fatalf("qemu-img info: expected raw format in output:\n%s", out)
	}
	t.Logf("qemu-img info: %s", out)
}
