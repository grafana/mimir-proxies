package convert

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"unsafe"

	"github.com/go-kit/log"
	"github.com/gogo/protobuf/proto"
)

var (
	ErrInvalidForMode = errors.New("the operation is not allowed under the mode")
	ErrBadSentinal    = errors.New("did not find end-of-file sentinel")
	ErrAtSentinel     = errors.New("file is complete, reads have been exhausted. File must only be Appended to")
	ErrBadData        = errors.New("invalid data in file")
)

const (
	sentinelCode = 0 // name length will be 0 to indicate the sentinel instead of a record.
	sentinel     = "SENTINEL"
)
const (
	READ = iota
	APPEND
)

type ErrSeek struct {
	Err error
}

func (e ErrSeek) Error() string {
	return fmt.Sprintf("error seeking in file: %s", e.Err.Error())
}

// USTable is an Unsorted String-keyed Table file containing unsorted proto
// items. An USTable file can either be opened for reading or appending.
//
// The file layout is:
//
//	repeated:
//	  int64 key length OR metadata code if <= 0
//	  key in bytes
//	  int64 proto length
//	  marshaled proto
//
// int64 0 value
// SENTINEL string indicating EOF
//
// Files without a SENTINEL are partial and would indicate incomplete
// processing.
// Futureproofing: negative values in the key length field can be used as an
// enum to indicate other metadata, such as error correction or an index.
type USTable struct {
	fname string
	fd    *os.File
	// The mode is either READ or APPEND.
	mode     int
	newValue ProtoConstructor
	// mu ensures that only one call to Append() or Next() is made at a time.
	mu sync.Mutex

	logger log.Logger
}

// ProtoUnmarshaler is similar to proto.Marshaler, except for the opposite
// direction.
type ProtoUnmarshaler interface {
	Unmarshal([]byte) error
}

// ProtoConstructor is a function which returns a new proto for this table's
// values. The return value from implementations should ideally be a pointer to
// a proto.
type ProtoConstructor func() ProtoUnmarshaler

// NewUSTableForAppend creates a new USTable file of the given name,
// creating one if it doesn't exist, and opening it if it does. If overwrite is
// true, the existing contents will be overwritten. If the file doesn't exist,
// it will be created regardless. If the file already existed, the write head
// will seek such as it will preserve existing valid contents.
func NewUSTableForAppend(fname string, overwrite bool, constructor ProtoConstructor, logger log.Logger) (*USTable, error) {
	i, created, err := newUSTableForAppend(fname, overwrite, constructor, logger)
	if err != nil {
		return nil, err
	}

	if !created {
		err = i.SeekLastValid()
	}
	return i, err
}

// NewUSTableForAppendWithIndex opens the file for appending, and also
// does a pre-read of the protos that are already written to the file.
func NewUSTableForAppendWithIndex(fname string, overwrite bool, constructor ProtoConstructor, logger log.Logger) (*USTable, map[string]int64, error) {
	i, created, err := newUSTableForAppend(fname, overwrite, constructor, logger)
	if err != nil {
		return nil, nil, err
	}

	index := make(map[string]int64)
	if !created {
		// Also seeks to the earliest valid position, but records a map of the protos
		// it sees along the way.  This can use a lot of memory for large USTable
		// files.
		i.mode = READ
		index, err = i.Index()
		if err != nil {
			return nil, nil, err
		}
		i.mode = APPEND
	}
	return i, index, err
}

// newUSTableForAppend is the implemetation of the constructor for
// appending to USTable files.  It does not do any seeking so the write
// head is at the beginning of the file.
func newUSTableForAppend(fname string, overwrite bool, constructor ProtoConstructor, logger log.Logger) (t *USTable, created bool, err error) {
	var fd *os.File
	if overwrite {
		fd, err = os.Create(fname)
		created = true
	} else {
		_, statErr := os.Stat(fname)
		if errors.Is(statErr, os.ErrNotExist) {
			fd, err = os.Create(fname)
			created = true
		} else {
			fd, err = os.OpenFile(fname, os.O_RDWR, os.ModePerm)
		}
	}
	if err != nil {
		return nil, false, err
	}

	t = &USTable{
		fname:    fname,
		fd:       fd,
		mode:     APPEND,
		newValue: constructor,
		logger:   logger,
	}

	return t, created, err
}

// NewUSTableForRead opens an USTable file for reading. Attempts to
// append will result in bad file descriptor errors.
func NewUSTableForRead(fname string, constructor ProtoConstructor, logger log.Logger) (*USTable, error) {
	var err error
	fd, err := os.OpenFile(fname, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}
	i := &USTable{
		fname:    fname,
		fd:       fd,
		mode:     READ,
		newValue: constructor,
		logger:   logger,
	}

	return i, nil
}

// Close closes the USTable file. If appending, a sentinel is written.
func (t *USTable) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mode == APPEND {
		err := t.writeSentinel()
		if err != nil {
			return err
		}
	}
	_ = t.fd.Close()
	return nil
}

// Append writes a new proto to the USTable file. This function assumes the
// file descriptor is at the end of the file. This function should not be used
// in conjunction with Next. Append() may be called concurrently.
func (t *USTable) Append(key string, value proto.Marshaler) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mode != APPEND {
		return ErrInvalidForMode
	}
	// Don't write anything until we confirm the value marshals
	valueBuf, err := value.Marshal()
	if err != nil {
		return err
	}

	// Write key length and value
	err = binary.Write(t.fd, binary.LittleEndian, int64(len(key)))
	if err != nil {
		return err
	}
	_, err = t.fd.WriteString(key)
	if err != nil {
		return err
	}

	// Write proto length and value
	err = binary.Write(t.fd, binary.LittleEndian, int64(len(valueBuf)))
	if err != nil {
		return err
	}
	_, err = t.fd.Write(valueBuf)
	return err
}

// SeekLastValid seeks to the end of the valid portion of the file, before any
// existing sentinel. Can be called in either Read or Append modes. Unmarshals
// values to confirm validity. Only returns error if there is an unrecoverable
// issue finding a valid position in the file.
func (t *USTable) SeekLastValid() (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// First try seeking to the end of the file to find the sentinel.
	if t.seekSentinel() {
		return nil
	}

	// That didn't work, so seek back to the beginning and find the last
	// valid data.
	_, err = t.fd.Seek(0, io.SeekStart)
	if err != nil {
		return ErrSeek{err}
	}

	var pos int64
	// If any error is encountered and it wasn't a seek error, try to seek back to
	// the previous known-good position.
	defer func() {
		if err != nil && !errors.Is(err, ErrSeek{}) {
			_, err = t.fd.Seek(pos, io.SeekStart)
			if err != nil {
				err = ErrSeek{err}
			}
		}
	}()

	for {
		pos, err = t.fd.Seek(0, io.SeekCurrent)
		if err != nil {
			return ErrSeek{err}
		}

		// Read the length at the current position so we know how much to skip,
		// or if we are at a sentinel.
		var nameLen int64
		err = binary.Read(t.fd, binary.LittleEndian, &nameLen)
		if err != nil {
			return
		}
		// This indicates we should be finding a sentinel.
		if nameLen == sentinelCode {
			// Even if the sentinel is malformed, it doesn't matter, seek back and we
			// are already at the place in the file we want to be.
			return ErrBadData
		}

		// Read string
		buf := make([]byte, nameLen)
		_, err = t.fd.Read(buf)
		if err != nil {
			return
		}

		// Read and unmarshal value data to make sure it's valid.
		_, err = t.readValue()
		if err != nil {
			return
		}
	}
}

// seekSentinel returns true if it was able to locate a valid sentinel and seek
// to its starting position (such that an append will overwrite the old
// sentinel).
func (t *USTable) seekSentinel() bool {
	// First try seeking to the end of the file to find the sentinel.
	var lenMarker int64
	atSentinelPos := 0 - (int64(unsafe.Sizeof(lenMarker)) + int64(len(sentinel)))
	_, err := t.fd.Seek(atSentinelPos, io.SeekEnd)
	if err == nil {
		err = binary.Read(t.fd, binary.LittleEndian, &lenMarker)
		if err == nil && lenMarker == sentinelCode {
			buf := make([]byte, len(sentinel))
			_, err = t.fd.Read(buf)
			if err == nil {
				if string(buf) == sentinel {
					// Re-seek to sentinel, we are good.
					_, err = t.fd.Seek(atSentinelPos, io.SeekEnd)
					return err != nil
				}
			}
		}
	}
	return false
}

// Index returns a map of key to position in the file.
// Performing a ReadAt for the given positions will read the value of the
// associated key.  When done, the read head will be at the next valid append
// point.
func (t *USTable) Index() (map[string]int64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mode != READ {
		return nil, ErrInvalidForMode
	}
	_, err := t.fd.Seek(0, io.SeekStart)
	if err != nil {
		return nil, ErrSeek{err}
	}

	records := make(map[string]int64)
	for {
		var pos int64
		pos, err = t.fd.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, ErrSeek{err}
		}
		var key string
		key, _, err = t.readAt(-1, false)
		if err != nil {
			// readAt will return the write head to an appropriate spot, so even if
			// there's an error we are done and it's fine, unless it was a seek
			// error.
			if errors.Is(err, ErrSeek{}) {
				return nil, err
			}
			break
		}
		records[key] = pos
	}

	return records, nil
}

// Next calls ReadAt for the current position.
func (t *USTable) Next() (key string, value ProtoUnmarshaler, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mode != READ {
		err = ErrInvalidForMode
		return
	}
	key, value, err = t.readAt(-1, true)
	return key, value, err
}

// ReadAt reads the record at the given file position and returns it. If the
// seek position is -1, uses the current position. In case of error, seeks back
// to the previous position before the call and no other calls to Next should be
// attempted (they will just fail again). If the file is complete, the last call
// to Next() will return nil and ErrAtSentinel. ReadAt() may be called
// concurrently, but if using seekPos -1 this may result in unpredictable
// behavior.
func (t *USTable) ReadAt(seekPos int64) (key string, value ProtoUnmarshaler, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mode != READ {
		err = ErrInvalidForMode
		return
	}
	key, value, err = t.readAt(seekPos, true)
	return key, value, err
}

// readAt is the implementation for reading record values. To save disk IO and
// CPU, it can optionally not read in the actual value and instead seek
// over it. The lock must be held before calling this function.  Some internal
// functions call this function repeatedly so it make sense to hold the lock
// higher up.
func (t *USTable) readAt(seekPos int64, readValue bool) (key string, value ProtoUnmarshaler, err error) {
	var recordStart int64
	if seekPos == -1 {
		recordStart, err = t.fd.Seek(0, io.SeekCurrent)
	} else {
		recordStart, err = t.fd.Seek(seekPos, io.SeekStart)
	}
	if err != nil {
		err = ErrSeek{err}
		return
	}

	// If any error is encountered, try to seek back to the previous known-good
	// position.
	defer func() {
		if err != nil {
			_, reseekErr := t.fd.Seek(recordStart, io.SeekStart)
			if reseekErr != nil {
				err = ErrSeek{reseekErr}
			}
		}
	}()

	var nameLen int64
	err = binary.Read(t.fd, binary.LittleEndian, &nameLen)
	if err != nil {
		return
	}
	// This indicates we should be finding a sentinel.
	if nameLen == sentinelCode {
		buf := make([]byte, len(sentinel))
		_, err = t.fd.Read(buf)
		if err != nil {
			return
		}
		if string(buf) != sentinel {
			err = ErrBadSentinal
		} else {
			err = ErrAtSentinel
		}
		// re-seek so we are at the position such that a write will overwrite
		// the sentinel and continue on.
		var seekErr error
		_, seekErr = t.fd.Seek(recordStart, io.SeekStart)
		if seekErr != nil {
			err = ErrSeek{seekErr}
		}
		return
	}

	// We've already got the name length so just read it in.
	buf := make([]byte, nameLen)
	_, err = t.fd.Read(buf)
	if err != nil {
		return
	}
	key = string(buf)

	if readValue {
		value, err = t.readValue()
	} else {
		// Seek past data to next record
		var valueLen int64
		err = binary.Read(t.fd, binary.LittleEndian, &valueLen)
		if err != nil {
			return
		}
		_, err = t.fd.Seek(valueLen, io.SeekCurrent)
		if err != nil {
			err = ErrSeek{err}
			return
		}
	}
	if err != nil {
		value = nil
	}
	return key, value, err
}

// readValue reads the proto from the current position in the file.
// It seeks back to pos on failure.
func (t *USTable) readValue() (ProtoUnmarshaler, error) {
	var valueLen int64
	err := binary.Read(t.fd, binary.LittleEndian, &valueLen)
	if err != nil {
		return nil, err
	}
	if valueLen < 0 {
		return nil, ErrBadData
	}
	if valueLen == 0 {
		return t.newValue(), nil
	}

	buf := make([]byte, valueLen)
	_, err = t.fd.Read(buf)
	if err != nil {
		return nil, err
	}

	value := t.newValue()
	err = value.Unmarshal(buf)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// writeSentinel writes an end-of-file sentinal.
func (t *USTable) writeSentinel() error {
	// Only called from Close, so lock is already held.
	err := binary.Write(t.fd, binary.LittleEndian, int64(0))
	if err != nil {
		return err
	}
	err = binary.Write(t.fd, binary.LittleEndian, []byte(sentinel))
	if err != nil {
		return err
	}
	return nil
}

func (t *USTable) pos() int64 {
	pos, err := t.fd.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(err)
	}
	return pos
}
