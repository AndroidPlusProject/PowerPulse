//go:build linux
package main

import (
	"fmt"
	"os"
)

func logMsg(logPriority LogPriority, msg string) {
	prio := "Unknown"
	switch logPriority {
	case LogDefault:
		prio = "*"
	case LogVerbose:
		if !verbose {
			return
		}
		prio = "V"
	case LogDebug:
		if !debug {
			return
		}
		prio = "D"
	case LogInfo:
		prio = "I"
	case LogWarn:
		prio = "W"
	case LogError:
		prio = "E"
	case LogFatal:
		prio = "F"
	case LogSilent:
		return
	}
	fmt.Printf("<%s> %s\n", prio, msg)
	if logPriority == LogFatal {
		os.Exit(1)
	}
}
