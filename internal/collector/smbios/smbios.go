// Package smbios implements a pure-Go SMBIOS/DMI table reader.
//
// Two read strategies are supported and selected automatically at runtime:
//   - sysfsReader  — preferred; reads /sys/firmware/dmi/tables/ (no root needed)
//   - devMemReader — fallback; reads /dev/mem by scanning 0xF0000–0xFFFFF for
//     the "_SM" / "_SM3" entry-point anchor (requires root / CAP_SYS_RAWIO)
//
// Parsed tables are cached process-wide via a sync.Once so that subsequent
// calls incur no I/O cost.
package smbios

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	sysfsDMI        = "/sys/firmware/dmi/tables/DMI"                // sysfs SMBIOS table blob
	sysfsEntryPoint = "/sys/firmware/dmi/tables/smbios_entry_point" // sysfs entry-point blob
	devMem          = "/dev/mem"                                    // physical memory device (fallback)
	startAddr       = 0xF0000                                       // SMBIOS scan range start (inclusive)
	endAddr         = 0x100000                                      // SMBIOS scan range end (exclusive)
	maxTableSize    = 1024 * 1024                                   // 1 MB upper bound on table size
	readTimeout     = 10 * time.Second                              // overall read deadline
)

// SMBIOSError wraps an SMBIOS I/O failure with the operation and path that
// caused it, allowing callers to distinguish SMBIOS errors from other errors.
type SMBIOSError struct {
	Op   string
	Path string
	Err  error
}

// Error implements the error interface.
func (e *SMBIOSError) Error() string {
	return fmt.Sprintf("smbios %s %s: %v", e.Op, e.Path, e.Err)
}

// Unwrap returns the underlying error to support errors.Is / errors.As.
func (e *SMBIOSError) Unwrap() error {
	return e.Err
}

// Reader is the internal interface that abstracts over the two SMBIOS read
// strategies (sysfs and /dev/mem).
type Reader interface {
	readTables(ctx context.Context, tableAddr, tableLen int) ([]*Table, error)
	readEntryPoint(ctx context.Context) (EntryPoint, error)
	Close() error
}

// sysfsReader reads SMBIOS data from the Linux sysfs DMI interface.
// It does not hold any resources and Close is a no-op.
type sysfsReader struct{}

// readTables reads the DMI table blob from /sys/firmware/dmi/tables/DMI.
func (r *sysfsReader) readTables(ctx context.Context, tableAddr, tableLen int) ([]*Table, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if tableLen <= 0 || tableLen > maxTableSize {
		return nil, fmt.Errorf("invalid table length: %d", tableLen)
	}

	file, err := os.Open(sysfsDMI)
	if err != nil {
		return nil, &SMBIOSError{Op: "open", Path: sysfsDMI, Err: err}
	}
	defer file.Close()

	return parseTables(file)
}

// readEntryPoint reads the SMBIOS entry point from
// /sys/firmware/dmi/tables/smbios_entry_point.
func (r *sysfsReader) readEntryPoint(ctx context.Context) (EntryPoint, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	file, err := os.Open(sysfsEntryPoint)
	if err != nil {
		return nil, &SMBIOSError{Op: "open", Path: sysfsEntryPoint, Err: err}
	}
	defer file.Close()

	return parseEntryPoint(file)
}

func (r *sysfsReader) Close() error {
	return nil // No resources to clean up
}

// devMemReader reads SMBIOS data directly from /dev/mem.
// It scans the physical memory range 0xF0000–0xFFFFF for the "_SM" anchor to
// locate the entry point, then reads the table at the address stored therein.
type devMemReader struct {
	file   *os.File
	mutex  sync.RWMutex
	closed bool
}

// NewDevMemReader opens /dev/mem and returns a devMemReader ready for use.
// The caller must call Close when finished.
func NewDevMemReader() (*devMemReader, error) {
	file, err := os.Open(devMem)
	if err != nil {
		return nil, &SMBIOSError{Op: "open", Path: devMem, Err: err}
	}

	return &devMemReader{file: file}, nil
}

// Close releases the /dev/mem file handle.
func (r *devMemReader) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.closed && r.file != nil {
		err := r.file.Close()
		r.file = nil
		r.closed = true
		return err
	}
	return nil
}

// isClosed is a thread-safe check on the closed flag.
func (r *devMemReader) isClosed() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.closed
}

// readTables reads tableLen bytes from /dev/mem at tableAddr and parses them
// as SMBIOS tables.
func (r *devMemReader) readTables(ctx context.Context, tableAddr, tableLen int) ([]*Table, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if tableAddr < 0 {
		return nil, fmt.Errorf("invalid table address: 0x%x", tableAddr)
	}
	if tableLen <= 0 || tableLen > maxTableSize {
		return nil, fmt.Errorf("invalid table length: %d", tableLen)
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.closed || r.file == nil {
		return nil, fmt.Errorf("reader is closed")
	}

	if _, err := r.file.Seek(int64(tableAddr), io.SeekStart); err != nil {
		return nil, &SMBIOSError{Op: "seek", Path: devMem, Err: err}
	}

	data := make([]byte, tableLen)
	if _, err := io.ReadFull(r.file, data); err != nil {
		return nil, &SMBIOSError{Op: "read", Path: devMem, Err: err}
	}

	return parseTables(bytes.NewReader(data))
}

// readEntryPoint locates and parses the SMBIOS entry point from /dev/mem.
func (r *devMemReader) readEntryPoint(ctx context.Context) (EntryPoint, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.closed || r.file == nil {
		return nil, fmt.Errorf("reader is closed")
	}

	epAddr, err := r.findEntryPointAddr(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := r.file.Seek(int64(epAddr), io.SeekStart); err != nil {
		return nil, &SMBIOSError{Op: "seek", Path: devMem, Err: err}
	}

	return parseEntryPoint(r.file)
}

// findEntryPointAddr scans the physical memory range 0xF0000–0xFFFFF in
// 16-byte (paragraph) increments looking for the "_SM" SMBIOS anchor.
// Returns the physical address of the entry point, or an error if not found.
func (r *devMemReader) findEntryPointAddr(ctx context.Context) (int, error) {

	if _, err := r.file.Seek(int64(startAddr), io.SeekStart); err != nil {
		return 0, &SMBIOSError{Op: "seek", Path: devMem, Err: err}
	}

	const paragraph = 16
	b := make([]byte, paragraph)

	for addr := startAddr; addr < endAddr; addr += paragraph {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		if _, err := io.ReadFull(r.file, b); err != nil {
			return 0, &SMBIOSError{Op: "read", Path: devMem, Err: err}
		}

		if bytes.HasPrefix(b, []byte("_SM")) {
			return addr, nil
		}
	}

	return 0, fmt.Errorf("SMBIOS entry point not found in memory range 0x%x-0x%x", startAddr, endAddr)
}

// smbiosReader is the top-level entry point that selects the appropriate read
// strategy and returns the parsed entry point and all SMBIOS tables.
// If sysfsEntryPoint exists the sysfsReader is used; otherwise /dev/mem is
// tried.  A default 10-second timeout is applied if ctx is nil.
func smbiosReader(ctx context.Context) (EntryPoint, []*Table, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
	}

	if _, err := os.Stat(sysfsEntryPoint); err == nil {
		reader := &sysfsReader{}
		return readFromSource(ctx, reader)
	}

	reader, err := NewDevMemReader()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create devmem reader: %w", err)
	}
	defer reader.Close()

	return readFromSource(ctx, reader)
}

// readFromSource reads the entry point and then the table data from the given
// Reader implementation, returning the parsed results.
func readFromSource(ctx context.Context, reader Reader) (EntryPoint, []*Table, error) {
	ep, err := reader.readEntryPoint(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read entry point: %w", err)
	}

	tableAddr, tableLen := ep.Table()
	tables, err := reader.readTables(ctx, tableAddr, tableLen)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read tables: %w", err)
	}

	return ep, tables, nil
}
