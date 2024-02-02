package main

import (
	"encoding/json"
)

type CPUFreq struct {
	Max json.Number
	Min json.Number
	Governor string
	Governors map[string]map[string]interface{} //"interactive":{"arg":0,"arg2":"val"},"performance":{"arg":true}
}
