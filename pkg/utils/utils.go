package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/shell"
)

// Scanner represents scanner for reading lines from an io.Reader.
type Scanner struct {
	scanner *bufio.Scanner
}

// NewScanner creates a new Scanner for reading from an io.Reader.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{
		scanner: bufio.NewScanner(r),
	}
}

// ParseLine parses a line into key-value from the scanner using the specified separator.
// It returns the key, value, and a boolean indicating if the scanner was fully parsed.
func (s *Scanner) ParseLine(sep string) (string, string, bool) {
	if !s.scanner.Scan() {
		return "", "", true
	}

	line := strings.TrimSpace(s.scanner.Text())
	k, v, found := strings.Cut(line, sep)
	if !found {
		return line, "", false
	}

	return strings.TrimSpace(k), strings.TrimSpace(v), false
}

// Err returns the error encountered during scanning(if any).
func (s *Scanner) Err() error {
	return s.scanner.Err()
}

// ReadLineOffsetN reads lines from a file starting at the specified offset and returns up to n lines.
// If n is negative or 0, it reads 64 lines from the offset.
// returns []string of lines and error if any.
func ReadLineOffsetN(path string, offset, n int64) ([]string, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", path, err)
	}
	defer file.Close()

	capcity := n
	if n <= 0 {
		capcity = 64 // default capcity
	}

	scanner := bufio.NewScanner(file)

	// skip lines until the offset
	for i := int64(0); i < offset; i++ {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("skip lines in %s: %w", path, err)
			}
			return []string{}, fmt.Errorf("file %s has less than %d lines", path, offset)
		}
	}

	lines := make([]string, 0, capcity)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
		if n > 0 && int64(len(lines)) >= n {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read lines in %s: %w", path, err)
	}

	return lines, nil
}

// ReadLine reads first line from a file.
// It returns trimspace str and error.
func ReadLine(path string) (string, error) {
	line, err := ReadLineOffsetN(path, 0, 1)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line[0]), nil
}

// unit index for parsing size
var unitIndex = map[string]int{
	"B":  0,
	"KB": 1, "KIB": 1, "K": 1,
	"MB": 2, "MIB": 2, "M": 2,
	"GB": 3, "GIB": 3, "G": 3,
	"TB": 4, "TIB": 4, "T": 4,
	"PB": 5, "PIB": 5, "P": 5,
	"EB": 6, "EIB": 6, "E": 6,
}

var unitNames = []string{"B", "KB", "MB", "GB", "TB", "PB"}

// ConvertUnit converts a size from one unit to another.
// binary is true for binary conversion, false for decimal conversion.
func ConvertUnit(size float64, fromUint, toUnit string, binary bool) (float64, error) {
	fromUpper := strings.ToUpper(strings.TrimSpace(fromUint))
	toUpper := strings.ToUpper(strings.TrimSpace(toUnit))

	fromIndex, ok := unitIndex[fromUpper]
	if !ok {
		return 0, fmt.Errorf("unsupported source unit: %s", fromUpper)
	}
	toIndex, ok := unitIndex[toUpper]
	if !ok {
		return 0, fmt.Errorf("unsupported target unit: %s", toUpper)
	}

	base := 1000.0
	if binary {
		base = 1024.0
	}

	diff := toIndex - fromIndex
	if diff > 0 {
		for range diff {
			size *= base
		}
	}

	if diff < 0 {
		for i := 0; i < -diff; i++ {
			size /= base
		}
	}

	return size, nil
}

// FormatSize formats a size with the specified unit.
// binary is true for binary conversion, false for decimal conversion.
func FormatSize(size float64, fromUnit, toUnit string, binary bool) (string, error) {
	result, err := ConvertUnit(size, fromUnit, toUnit, binary)
	if err != nil {
		return "", err
	}

	if result == float64(int64(result)) {
		return fmt.Sprintf("%d %s", int64(result), toUnit), nil
	}
	return fmt.Sprintf("%.2f %s", result, toUnit), nil
}

// AutoFormatSize formats a size with the appropriate unit.
func AutoFormatSize(size float64, fromUnit string, binary bool) string {
	fromUpper := strings.ToUpper(strings.TrimSpace(fromUnit))
	fromIndex, ok := unitIndex[fromUpper]
	if !ok {
		return fmt.Sprintf("%.2f %s", size, fromUpper)
	}

	base := 1000.0
	if binary {
		base = 1024.0
	}

	bytes := size
	for range fromIndex {
		bytes *= base
	}

	idx := 0
	for bytes >= base && idx < len(unitNames)-1 {
		bytes /= base
		idx++
	}

	if bytes == float64(int64(bytes)) {
		return fmt.Sprintf("%d %s", int64(bytes), unitNames[idx])
	}

	return fmt.Sprintf("%.2f %s", bytes, unitNames[idx])
}

func FillField(s string, t *string) {
	if s == "" || *t != "" {
		return
	}

	*t = s
}

// ReadLinkBase 读取符号链接的基本名称
// 该函数首先通过 os.Readlink 获取符号链接的目标路径，然后使用 filepath.Base 提取该路径的基本名称（最后一部分）
// path: 符号链接的路径
// 返回值: 符号链接目标的基本名称和可能的错误
func ReadLinkBase(path string) (string, error) {
	link, err := os.Readlink(path)
	if err != nil {
		return "", err
	}

	return filepath.Base(link), nil
}

func GetBlockByLsblk() []string {
	output, err := shell.Run("lsblk", "-d", "-o", "NAME", "-n")
	if err != nil {
		return nil
	}

	lines := bytes.Split(output, []byte("\n"))
	blocks := make([]string, 0, len(lines))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		blocks = append(blocks, string(line))
	}
	return blocks
}

const (
	wwnPrefix  = "wwn-"
	partSuffix = "-part"
)

func GetBlockByWWN(wwn string) string {
	files, err := os.ReadDir(paths.DevDiskByID)
	if err != nil {
		return ""
	}

	for _, f := range files {

		fn := f.Name()
		if !strings.HasPrefix(fn, wwnPrefix) || strings.Contains(fn, partSuffix) {
			continue
		}

		if idx := strings.IndexByte(fn, '-'); idx != -1 {
			if wwn != fn[idx+1:] {
				continue
			}
		}

		if value, err := ReadLinkBase(fn); err == nil {
			return value
		}
	}

	return ""
}

func GetBlockFromSysfs() []string {
	devices, err := os.ReadDir(paths.SysBlock)
	if err != nil {
		return GetBlockByLsblk()
	}

	blocks := make([]string, 0, len(devices))
	for _, device := range devices {
		name := device.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "md") {
			continue
		}
		blocks = append(blocks, name)
	}

	return blocks
}

// IsEmpty 判断 reflect.Value 是否为空
func IsEmpty(v reflect.Value) bool {
	// 检查 Value 是否有效
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map, reflect.Chan:
		return v.IsNil() || v.Len() == 0
	case reflect.Array:
		return v.Len() == 0
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Struct:
		// 遍历结构体所有字段，判断是否全为空
		for i := 0; i < v.NumField(); i++ {
			if !IsEmpty(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
