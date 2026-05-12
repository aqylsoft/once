package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"context"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/aqylsoft/once"
)

var (
	globalStore *once.MemoryStore
	storeMu     sync.Mutex

	locksMu    sync.Mutex
	locks      = make(map[int]func())
	nextLockID int
)

//export once_init
func once_init() {
	storeMu.Lock()
	defer storeMu.Unlock()

	if globalStore == nil {
		globalStore = once.NewMemoryStore()
	}
}

//export once_destroy
func once_destroy() {
	if globalStore != nil {
		globalStore.Stop()
		globalStore = nil
	}

	locksMu.Lock()
	for id, unlock := range locks {
		unlock()
		delete(locks, id)
	}
	locksMu.Unlock()
}

//export once_check
// Returns: 0=not found, 1=found (fills response buffer), -1=locked (conflict)
func once_check(key *C.char, response *C.char, responseLen C.int) C.int {
	if globalStore == nil {
		return -1
	}

	goKey := C.GoString(key)
	ctx := context.Background()

	resp, found := globalStore.Get(ctx, goKey)
	if !found {
		return 0
	}

	if response != nil && responseLen > 0 {
		bodyLen := len(resp.Body)
		if bodyLen > int(responseLen)-1 {
			bodyLen = int(responseLen) - 1
		}
		if bodyLen > 0 {
			C.memcpy(unsafe.Pointer(response), unsafe.Pointer(&resp.Body[0]), C.size_t(bodyLen))
		}
		*(*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(response)) + uintptr(bodyLen))) = 0
	}

	return 1
}

//export once_check_full
// Returns status code if found (>0), 0=not found, -1=error
// Fills response buffer with body, returns actual body length in bodyLen
func once_check_full(key *C.char, statusCode *C.int, response *C.char, responseMaxLen C.int, bodyLen *C.int) C.int {
	if globalStore == nil {
		return -1
	}

	goKey := C.GoString(key)
	ctx := context.Background()

	resp, found := globalStore.Get(ctx, goKey)
	if !found {
		return 0
	}

	if statusCode != nil {
		*statusCode = C.int(resp.StatusCode)
	}

	if bodyLen != nil {
		*bodyLen = C.int(len(resp.Body))
	}

	if response != nil && responseMaxLen > 0 {
		copyLen := len(resp.Body)
		if copyLen > int(responseMaxLen)-1 {
			copyLen = int(responseMaxLen) - 1
		}
		if copyLen > 0 {
			C.memcpy(unsafe.Pointer(response), unsafe.Pointer(&resp.Body[0]), C.size_t(copyLen))
		}
		*(*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(response)) + uintptr(copyLen))) = 0
	}

	return 1
}

//export once_store
// Stores a response. Returns 0 on success, -1 on error.
func once_store(key *C.char, statusCode C.int, body *C.char, bodyLen C.int, ttlSeconds C.int) C.int {
	if globalStore == nil {
		return -1
	}

	goKey := C.GoString(key)
	ctx := context.Background()

	var goBody []byte
	if body != nil && bodyLen > 0 {
		goBody = C.GoBytes(unsafe.Pointer(body), bodyLen)
	}

	resp := &once.Response{
		StatusCode: int(statusCode),
		Headers:    make(http.Header),
		Body:       goBody,
		CreatedAt:  time.Now(),
	}

	ttl := time.Duration(ttlSeconds) * time.Second
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	err := globalStore.Set(ctx, goKey, resp, ttl)
	if err != nil {
		return -1
	}

	return 0
}

//export once_lock
// Acquires a lock. Returns lock_id (>=0) on success, -1 if already locked.
func once_lock(key *C.char) C.int {
	if globalStore == nil {
		return -1
	}

	goKey := C.GoString(key)
	ctx := context.Background()

	unlock, err := globalStore.Lock(ctx, goKey)
	if err != nil {
		return -1
	}

	locksMu.Lock()
	id := nextLockID
	nextLockID++
	locks[id] = unlock
	locksMu.Unlock()

	return C.int(id)
}

//export once_unlock
// Releases a lock by lock_id.
func once_unlock(lockId C.int) {
	locksMu.Lock()
	unlock, exists := locks[int(lockId)]
	if exists {
		delete(locks, int(lockId))
	}
	locksMu.Unlock()

	if unlock != nil {
		unlock()
	}
}

func main() {}
