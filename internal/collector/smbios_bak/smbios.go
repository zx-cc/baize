package smbios

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zx-cc/baize/pkg/paths"
)

const (
	headerLength = 4
	anchor32     = "_SM_"
	anchor64     = "_SM3_"
	anchor32Len  = 0x1f
	anchor64Len  = 0x18
)

type Header struct {
	Type   uint8
	Length uint8
	Handle uint16
}

type Table struct {
	Header        Header
	FormattedArea []byte
	StringArea    []string
}

type EntryPoint interface {
	Table() (int, int)
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
}

type SMBIOS struct {
	EntryPoint EntryPoint
	Tables     []Table
}

func New() (*SMBIOS, error) {
	smbios, err := readFromSysfs()
	fmt.Println("readFromSysfs", err)
	if err == nil {
		return smbios, nil
	}

	return readFromDevMem()
}

func readFromSysfs() (*SMBIOS, error) {
	entryPoint, err := os.Open(filepath.Join(paths.SysFirmwareDmiTables, "smbios_entry_point"))
	if err != nil {
		return nil, fmt.Errorf("open smbios_entry_point: %w", err)
	}
	defer entryPoint.Close()

	dmi, err := os.Open(filepath.Join(paths.SysFirmwareDmiTables, "DMI"))
	if err != nil {
		return nil, fmt.Errorf("open DMI: %w", err)
	}
	defer dmi.Close()

	var errs []error
	ep, err := parseEntryPoint(entryPoint)
	if err != nil {
		errs = append(errs, fmt.Errorf("parse entry point: %w", err))
	}

	tables, err := parseTables(dmi)
	if err != nil {
		errs = append(errs, fmt.Errorf("parse tables: %w", err))
	}

	return &SMBIOS{
		EntryPoint: ep,
		Tables:     tables,
	}, errors.Join(errs...)
}

func readFromDevMem() (*SMBIOS, error) {
	file, err := os.Open(paths.DevMem)
	if err != nil {
		return nil, fmt.Errorf("open /dev/mem: %w", err)
	}
	defer file.Close()

	// 搜索入口点：范围 0xF0000-0xFFFFF
	const searchStart = 0xF0000
	const searchEnd = 0x100000
	const paragraphSize = 16

	if _, err := file.Seek(searchStart, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek start /dev/mem: %w", err)
	}
	searchArea := make([]byte, paragraphSize)
	for addr := searchStart; addr < searchEnd; addr += paragraphSize {
		if _, err := io.ReadFull(file, searchArea); err != nil {
			return nil, fmt.Errorf("read /dev/mem: %w", err)
		}
		if bytes.HasPrefix(searchArea, []byte("_SM")) {
			if _, err := file.Seek(int64(addr), io.SeekStart); err != nil {
				return nil, fmt.Errorf("seek address /dev/mem: %w", err)
			}
			break
		}
	}

	ep, err := parseEntryPoint(file)
	if err != nil {
		return nil, fmt.Errorf("parse entry point: %w", err)
	}

	tableAddr, tableLen := ep.Table()
	if tableAddr <= 0 || tableLen <= 0 {
		return nil, fmt.Errorf("invalid table address or length")
	}

	if _, err := file.Seek(int64(tableAddr), io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek table address /dev/mem: %w", err)
	}

	data := make([]byte, tableLen)
	if _, err := io.ReadFull(file, data); err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	tables, err := parseTables(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse tables: %w", err)
	}

	return &SMBIOS{
		EntryPoint: ep,
		Tables:     tables,
	}, nil
}

func parseEntryPoint(ir io.Reader) (EntryPoint, error) {
	bf := bufio.NewReader(ir)
	peek, err := bf.Peek(5)
	if err != nil {
		return nil, err
	}

	var (
		ep   EntryPoint
		data []byte
	)

	switch {
	case bytes.Equal(peek[:4], []byte(anchor32)):
		data = make([]byte, 0, anchor32Len)
		ep = &entryPoint32{}
	case bytes.Equal(peek, []byte(anchor64)):
		data = make([]byte, 0, anchor64Len)
		ep = &entryPoint64{}
	default:
		return nil, fmt.Errorf("invalid anchor string: %v", peek[:])
	}

	if _, err := io.ReadFull(bf, data); err != nil {
		return nil, fmt.Errorf("read entry point: %w", err)
	}

	if err := ep.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("unmarshal entry point: %w", err)
	}

	return ep, nil
}

func parseTables(ir io.Reader) ([]Table, error) {
	var tables []Table
	br := bufio.NewReader(ir)
	for {
		if _, err := br.Peek(1); err == io.EOF {
			return tables, nil
		}

		t, err := parseTable(br)
		if err != nil {
			return nil, fmt.Errorf("parse table: %w", err)
		}
		tables = append(tables, t)
	}
}
