//go:build android
package main

import (
	"unsafe"
)

// #cgo LDFLAGS: -llog
// #include <android/log.h>
// #include <stdlib.h>
// #include <string.h>
import "C"

func logMsg(prio LogPriority, msg string) {
	ctag := C.CString(LOG_TAG)
	cstr := C.CString(msg)
	cprio := C.int(prio)
	C.__android_log_write(cprio, ctag, cstr)
	C.free(unsafe.Pointer(ctag))
	C.free(unsafe.Pointer(cstr))
}