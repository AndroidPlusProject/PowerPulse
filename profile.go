package main

import (
	"encoding/json"
)

type Profile struct {
	Clusters map[string]*Cluster
	CPUSets *CPUSets
	GPU *GPU
	Kernel *Kernel
	IPA *IPA
	InputBooster *InputBooster
	SecSlow *SecSlow
}

type Cluster struct {
	CPUFreq *CPUFreq
}

type CPUSets struct {
	Foreground *CPUSet
	SystemBackground *CPUSet `json:"system_background"`
	Background *CPUSet
	TopApp *CPUSet `json:"top_app"`
	Restricted *CPUSet
}

type CPUSet struct {
	CPUs string
	CPUExclusive string `json:"cpu_exclusive"`
}

type GPU struct {
	DVFS *DVFS
	Highspeed *GPUHighspeed
}

type DVFS struct {
	Min json.Number
	Max json.Number
}

type GPUHighspeed struct {
	Clock json.Number
	Load json.Number
}

type Kernel struct {
	DynamicHotplug bool
	PowerEfficient bool
	HMP *KernelHMP
}

type KernelHMP struct {
	Boost bool
	Semiboost bool
	ActiveDownMigration bool
	AggressiveUpMigration bool
	Threshold *KernelHMPThreshold
	SbThreshold *KernelHMPThreshold
}

type KernelHMPThreshold struct {
	Down json.Number
	Up json.Number
}

type IPA struct {
	Enabled bool
	ControlTemp json.Number
}

type InputBooster struct {
	Head string
	Tail string
}

type SecSlow struct {
	Enabled bool
	Enforced bool
	TimerRate json.Number
}
