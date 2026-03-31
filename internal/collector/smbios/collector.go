package smbios

import (
	"fmt"
	"sync"
)

type Collector struct {
	smbios *SMBIOS
	err    error
	once   sync.Once
	mu     sync.RWMutex
}

var defaultCollector = &Collector{}

func (c *Collector) collect() {
	c.once.Do(func() {
		c.smbios, c.err = New()
	})
}

func (c *Collector) Get() (*SMBIOS, error) {
	c.collect()
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.smbios, c.err
}

func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.once = sync.Once{}
	c.smbios = nil
	c.err = nil
}

func (c *Collector) IsCollected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.smbios != nil
}

func Collect() (*SMBIOS, error) {
	return defaultCollector.Get()
}

func Reset() {
	defaultCollector.Reset()
}

type parseFunc func(*Table) (any, error)

var typeParser = map[uint8]parseFunc{
	TypeBIOSInfo:      func(t *Table) (any, error) { return parseType0BIOS(t) },
	TypeSystemInfo:    func(t *Table) (any, error) { return parseType1System(t) },
	TypeBaseboardInfo: func(t *Table) (any, error) { return parseType2BaseBoard(t) },
	TypeChassisInfo:   func(t *Table) (any, error) { return parseType3Chassis(t) },
	TypeProcessorInfo: func(t *Table) (any, error) { return parseType4Processor(t) },
	TypeMemoryDevice:  func(t *Table) (any, error) { return parseType17MemoryDevice(t) },
}

func GetTypeData[T any](t uint8) ([]T, error) {
	s, err := Collect()
	if err != nil {
		return nil, err
	}

	tp, exists := typeParser[t]
	if !exists {
		return nil, fmt.Errorf("not supported table type: %d", t)
	}

	var res []T

	for _, tb := range s.Tables {
		if tb.Header.Type == t {
			data, err := tp(&tb)
			if data != nil && err == nil {
				res = append(res, data.(T))
			}
		}
	}

	return res, nil
}
