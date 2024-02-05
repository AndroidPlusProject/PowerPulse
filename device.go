package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"
)

type Device struct {
	sync.Mutex

	Buffered            []BufferedWrite                            //Buffered string values ready to be synced to each path
	Paths               *Paths                                     //Manifest of paths to device settings
	ProfileBoot         string      `json:"profile_boot"`          //Default profile, used permanently without a profile manager
	ProfileBootDuration json.Number `json:"profile_boot_duration"` //Force sets the boot profile for X seconds (no decimals) before setting the first requested profile after init
	ProfileInheritance  []string    `json:"profile_inheritance"`   //Profile order for inheritance of configurations
	ProfileOrder        []string    `json:"profile_order"`         //Profile order for stargazing
	Profiles            map[string]*Profile                        //Manifest of device settings per profile
}

type BufferedWrite struct {
	Path string
	Data string
}

func (dev *Device) CacheProfile(name string) error {
	found := false
	for i := 0; i < len(dev.ProfileOrder); i++ {
		if dev.ProfileOrder[i] == name {
			found = true
			break
		}
	}
	if !found {
		return nil //Silently fail, we only want to cache profiles we can pick from
	}
	if dev.Paths.PowerPulse != nil && dev.Paths.PowerPulse.Profile != "" {
		return dev.write(dev.Paths.PowerPulse.Profile, name)
	}
	return nil
}

func (dev *Device) SyncProfile() error {
	Debug("Syncing profile")
	if dev.Buffered == nil {return nil}
	for i := 0; i < len(dev.Buffered); i++ {
		bw := dev.Buffered[i]
		if err := dev.write(bw.Path, bw.Data); err != nil {return err}
	}
	//Reset the buffer for the next profile chain
	dev.Buffered = make([]BufferedWrite, 0)
	return nil
}

func (dev *Device) GetProfile(name string) *Profile {
	return dev.Profiles[name]
}

func (dev *Device) SetProfile(name string) error {
	dev.Lock()
	defer dev.Unlock()

	startTime := time.Now()
	profile := dev.GetProfile(name)
	if profile == nil {
		return fmt.Errorf("profile %s does not exist", name)
	}

	//Inherit any parent profiles if available
	index := -1
	for i := 0; i < len(dev.ProfileInheritance); i++ {
		if dev.ProfileInheritance[i] == name {
			index = i
			break
		}
	}

	//Skip inheritance if we are the parent profile
	if index > 0 {
		for i := 0; i < index; i++ {
			parentName := dev.ProfileInheritance[i]
			parent := dev.GetProfile(parentName)
			if parent == nil {
				return fmt.Errorf("failed to get profile %s from profile order for inheritance, missed validation", parentName)
			}
			if err := dev.setProfile(parent); err != nil {return err}
		}
	}

	//Set the new profile and do it live
	if err := dev.setProfile(profile); err != nil {return err}
	if err := dev.SyncProfile(); err != nil {return err}

	//Handle cpusets separately for safety reasons
	if err := dev.setCpusets(index, profile); err != nil {return err}

	deltaTime := time.Now().Sub(startTime).Milliseconds()
	Info("PowerPulse finished applying %s in %dms", name, deltaTime)
	return nil
}

func (dev *Device) setProfile(profile *Profile) error {
	for clusterName, cluster := range profile.Clusters {
		pathCluster := dev.Paths.Clusters[clusterName]
		clusterPath := pathCluster.Path
		if debug {
			Debug("Loading CPU cluster %s", clusterName)
			Debug(clusterPath)
		}

		if cluster.CPUFreq != nil {
			freq := cluster.CPUFreq
			pathFreq := dev.Paths.Clusters[clusterName].CPUFreq
			freqPath := pathJoin(clusterPath, pathFreq.Path)
			if debug {
				Debug("Loading cpufreq %s", freqPath)
				Debug(freqPath)
			}
			if freq.Governor != "" {
				governorPath := pathJoin(freqPath, pathFreq.Governor)
				if debug {
					Debug("> CPUFreq > Governor = %s", freq.Governor)
					Debug(governorPath)
				}
				dev.BufferWrite(governorPath, freq.Governor)
			}
			max := freq.Max.String()
			if max != "" {
				Debug("> CPUFreq > Max = %s", max)
				maxPath := pathJoin(freqPath, pathFreq.Max)
				Debug(maxPath)
				dev.BufferWrite(maxPath, max)
			}
			min := freq.Min.String()
			if min != "" {
				minPath := pathJoin(freqPath, pathFreq.Min)
				if debug {
					Debug("> CPUFreq > Min = %s", min)
					Debug(minPath)
				}
				dev.BufferWrite(minPath, min)
			}
			speed := freq.Speed.String()
			if speed != "" {
				speedPath := pathJoin(freqPath, pathFreq.Speed)
				if debug {
					Debug("> CPUFreq > Speed = %s", speed)
					Debug(speedPath)
				}
				dev.BufferWrite(speedPath, speed)
			}
			for governorName, governor := range freq.Governors {
				governorPath := pathJoin(freqPath, governorName)
				if debug {
					Debug("Loading cpufreq governor %s", governorName)
					Debug(governorPath)
				}
				for arg, val := range governor {
					argPath := pathJoin(governorPath, arg)
					Debug(argPath)
					switch v := val.(type) {
					case bool:
						Debug("> %s > %s = %t", governorName, arg, v)
						if err := dev.BufferWriteBool(argPath, v); err != nil {return err}
					case float64:
						Debug("> %s > %s = %.0F", governorName, arg, v)
						dev.BufferWriteNumber(argPath, v)
					case string:
						Debug("> %s > %s = %s", governorName, arg, v)
						dev.BufferWrite(argPath, v)
					default:
						return fmt.Errorf("governor %s has invalid value type '%T' for arg %s", governorName, v, arg)
					}
				}
			}
		}
	}

	if profile.GPU != nil {
		gpu := profile.GPU
		gpuPath := dev.Paths.GPU.Path
		if debug {
			Debug("Loading GPU")
			Debug(gpuPath)
		}
		if gpu.DVFS != nil {
			dvfs := gpu.DVFS
			Debug("Loading GPU DVFS")
			max := dvfs.Max.String()
			if max != "" {
				maxPath := pathJoin(gpuPath, dev.Paths.GPU.DVFS.Max)
				if debug {
					Debug("> GPU > DVFS > Max = %s", max)
					Debug(maxPath)
				}
				dev.BufferWrite(maxPath, max)
			}
			min := dvfs.Min.String()
			if min != "" {
				minPath := pathJoin(gpuPath, dev.Paths.GPU.DVFS.Min)
				if debug {
					Debug("> GPU > DVFS > Min = %s", min)
					Debug(minPath)
				}
				dev.BufferWrite(minPath, min)
			}
		}
		if gpu.Highspeed != nil {
			hs := gpu.Highspeed
			Debug("Loading GPU highspeed")
			clock := hs.Clock.String()
			if clock != "" {
				clockPath := pathJoin(gpuPath, dev.Paths.GPU.Highspeed.Clock)
				if debug {
					Debug("> GPU > Highspeed > Clock = %s", clock)
					Debug(clockPath)
				}
				dev.BufferWrite(clockPath, clock)
			}
			load := hs.Load.String()
			if load != "" {
				loadPath := pathJoin(gpuPath, dev.Paths.GPU.Highspeed.Load)
				if debug {
					Debug("> GPU > Highspeed > Load = %s", load)
					Debug(loadPath)
				}
				dev.BufferWrite(loadPath, load)
			}
		}
	}

	if profile.Kernel != nil {
		krnl := profile.Kernel
		Debug("Loading kernel")
		dynamicHotplugPath := dev.Paths.Kernel.DynamicHotplug
		if debug {
			Debug("> Kernel > Dynamic Hotplug = %t", krnl.DynamicHotplug)
			Debug(dynamicHotplugPath)
		}
		if err := dev.BufferWriteBool(dynamicHotplugPath, krnl.DynamicHotplug); err != nil {return err}
		powerEfficientPath := dev.Paths.Kernel.PowerEfficient
		if debug {
			Debug("> Kernel > Power Efficient = %t", krnl.PowerEfficient)
			Debug(powerEfficientPath)
		}
		if err := dev.BufferWriteBool(powerEfficientPath, krnl.PowerEfficient); err != nil {return err}
		if krnl.HMP != nil {
			hmp := krnl.HMP
			hmpPath := dev.Paths.Kernel.HMP.Path
			if debug {
				Debug("Loading kernel HMP")
				Debug(hmpPath)
			}
			hmpPaths := dev.Paths.Kernel.HMP
			boostPath := pathJoin(hmpPath, hmpPaths.Boost)
			if debug {
				Debug("> Kernel > HMP > Boost = %t", hmp.Boost)
				Debug(boostPath)
			}
			if err := dev.BufferWriteBool(boostPath, hmp.Boost); err != nil {return err}
			semiboostPath := pathJoin(hmpPath, hmpPaths.Semiboost)
			if debug {
				Debug("> Kernel > HMP > Semiboost = %t", hmp.Semiboost)
				Debug(semiboostPath)
			}
			if err := dev.BufferWriteBool(semiboostPath, hmp.Semiboost); err != nil {return err}
			activeDownMigrationPath := pathJoin(hmpPath, hmpPaths.ActiveDownMigration)
			if debug {
				Debug("> Kernel > HMP > Active Down Migration = %t", hmp.ActiveDownMigration)
				Debug(activeDownMigrationPath)
			}
			if err := dev.BufferWriteBool(activeDownMigrationPath, hmp.ActiveDownMigration); err != nil {return err}
			aggressiveUpMigrationPath := pathJoin(hmpPath, hmpPaths.AggressiveUpMigration)
			if debug {
				Debug("> Kernel > HMP > Aggressive Up Migration = %t", hmp.AggressiveUpMigration)
				Debug(aggressiveUpMigrationPath)
			}
			if err := dev.BufferWriteBool(aggressiveUpMigrationPath, hmp.AggressiveUpMigration); err != nil {return err}
			if hmp.Threshold != nil {
				thld := hmp.Threshold
				down := thld.Down.String()
				if down != "" {
					downPath := pathJoin(hmpPath, hmpPaths.Threshold.Down)
					if debug {
						Debug("> Kernel > HMP > Threshold > Down = %s", down)
						Debug(downPath)
					}
					dev.BufferWrite(downPath, down)
				}
				up := thld.Up.String()
				if up != "" {
					upPath := pathJoin(hmpPath, hmpPaths.Threshold.Up)
					if debug {
						Debug("> Kernel > HMP > Threshold > Up = %s", up)
						Debug(upPath)
					}
					dev.BufferWrite(upPath, up)
				}
			}
			if hmp.SbThreshold != nil {
				thld := hmp.SbThreshold
				down := thld.Down.String()
				if down != "" {
					downPath := pathJoin(hmpPath, hmpPaths.SbThreshold.Down)
					if debug {
						Debug("> Kernel > HMP > Semiboost Threshold > Down = %s", down)
						Debug(downPath)
					}
					dev.BufferWrite(downPath, down)
				}
				up := thld.Up.String()
				if up != "" {
					upPath := pathJoin(hmpPath, hmpPaths.SbThreshold.Up)
					if debug {
						Debug("> Kernel > HMP > Semiboost Threshold > Up = %s", up)
						Debug(upPath)
					}
					dev.BufferWrite(upPath, up)
				}
			}
		}
	}

	if profile.IPA != nil {
		ipa := profile.IPA
		ipaPaths := dev.Paths.IPA
		ipaPath := ipaPaths.Path
		enabledPath := pathJoin(ipaPath, ipaPaths.Enabled)
		if debug {
			Debug("Loading IPA")
			Debug("> IPA > Enabled = %t", ipa.Enabled)
			Debug(enabledPath)
		}
		if err := dev.BufferWriteBool(enabledPath, ipa.Enabled); err != nil {return err}
		if ipa.Enabled {
			controlTemp := ipa.ControlTemp.String()
			if controlTemp != "" {
				ctPath := pathJoin(ipaPath, ipaPaths.ControlTemp)
				if debug {
					Debug("> IPA > Control Temp = %s", controlTemp)
					Debug(ctPath)
				}
				dev.BufferWrite(ctPath, controlTemp)
			}
		}
	}

	if profile.InputBooster != nil {
		ib := profile.InputBooster
		ibPaths := dev.Paths.InputBooster
		if debug {
			Debug("Loading input booster")
		}
		if ib.Head != "" {
			headPath := ibPaths.Head
			if debug {
				Debug("> Input Booster > Head = %s", ib.Head)
				Debug(headPath)
			}
			dev.BufferWrite(headPath, ib.Head)
		}
		if ib.Tail != "" {
			tailPath := ibPaths.Tail
			if debug {
				Debug("> Input Booster > Tail = %s", ib.Tail)
				Debug(tailPath)
			}
			dev.BufferWrite(tailPath, ib.Tail)
		}
	}

	if profile.SecSlow != nil {
		slow := profile.SecSlow
		slowPaths := dev.Paths.SecSlow
		enabledPath := slowPaths.Enabled
		if debug {
			Debug("Loading sec_slow")
			Debug("> sec_slow > Enabled = %t", slow.Enabled)
			Debug(enabledPath)
		}
		if err := dev.BufferWriteBool(enabledPath, slow.Enabled); err != nil {return err}
		if slow.Enabled {
			enforcedPath := slowPaths.Enforced
			if debug {
				Debug("> sec_slow > Enforced = %t", slow.Enforced)
				Debug(enforcedPath)
			}
			if err := dev.BufferWriteBool(enforcedPath, slow.Enforced); err != nil {return err}
			timerRate := slow.TimerRate.String()
			if timerRate != "" {
				timerRatePath := slowPaths.TimerRate
				if debug {
					Debug("> sec_slow > Timer Rate = %s", timerRate)
					Debug(timerRatePath)
				}
				dev.BufferWrite(timerRatePath, timerRate)
			}
		}
	}

	return nil
}

func (dev *Device) ReadBool(path string) (bool, error) {
	buffer, err := ioutil.ReadFile(path)
	if err != nil {
		return false, err
	} else {
		if buffer[len(buffer)-1] == '\n' { buffer = buffer[:len(buffer)-1] }
	}
	switch string(buffer) {
	case "1", "t", "T", "true", "True", "TRUE", "y", "Y", "yes", "Yes", "YES", "enabled", "Enabled", "ENABLED":
		return true, nil
	case "0", "f", "F", "false", "False", "FALSE", "n", "N", "no", "No", "NO", "disabled", "Disabled", "DISABLED":
		return false, nil
	}
	return false, fmt.Errorf("Unknown bool interface for %s", path)
}

func (dev *Device) setCpusets(index int, profile *Profile) error {
	//Skip inheritance if we are the parent cpuset
	if index > 0 {
		for i := 0; i < index; i++ {
			parentName := dev.ProfileInheritance[i]
			parent := dev.GetProfile(parentName)
			if parent == nil {
				return fmt.Errorf("failed to get profile %s from profile order for inheritance, missed validation", parentName)
			}
			if err := dev.setCpusetsBuffered(parent, false); err != nil {return err}
		}
	}
	//Set the new cpusets and do it live
	return dev.setCpusetsBuffered(profile, true)
}

func (dev *Device) setCpusetsBuffered(profile *Profile, flush bool) error {
	if profile.CPUSets != nil {
		sets := profile.CPUSets
		setsPath := dev.Paths.Cpusets.Path
		if debug {
			Debug("Loading cpusets")
			Debug(setsPath)
		}

		//To start, clear any old CPU exclusives
		for setName, _ := range sets {
			setPath := pathJoin(setsPath, setName)
			if debug {
				Debug("> CPUSets > %s > Exclusive CPU = False (temporarily)", setName)
				Debug(setPath)
			}
			exclusivePath := pathJoin(setPath, dev.Paths.Cpusets.Sets[setName].CPUExclusive)
			if err := dev.BufferWriteBool(exclusivePath, false); err != nil {return err}
		}
		if flush {dev.SyncProfile()}

		//Set up the new CPU sets
		for setName, set := range sets {
			setPath := pathJoin(setsPath, setName)
			if debug {
				Debug("> CPUSets > %s > CPUs = %s", setName, set.CPUs)
				Debug(setPath)
			}
			cpusPath := pathJoin(setPath, dev.Paths.Cpusets.Sets[setName].CPUs)
			dev.BufferWrite(cpusPath, set.CPUs)
		}
		if flush {dev.SyncProfile()}

		//Finally, set up any new CPU exclusives
		for setName, set := range sets {
			setPath := pathJoin(setsPath, setName)
			if debug {
				Debug("> CPUSets > %s > Exclusive CPU = %t", setName, set.CPUExclusive)
				Debug(setPath)
			}
			exclusivePath := pathJoin(setPath, dev.Paths.Cpusets.Sets[setName].CPUExclusive)
			if err := dev.BufferWriteBool(exclusivePath, set.CPUExclusive); err != nil {return err}
		}
		if flush {dev.SyncProfile()}
	}
	return nil
}

func (dev *Device) BufferWriteBool(path string, data bool) error {
	buffer, err := ioutil.ReadFile(path)
	if err != nil {
		Warn("Failed to read from path %s: %v", path, err)
	} else {
		if buffer[len(buffer)-1] == '\n' { buffer = buffer[:len(buffer)-1] }
	}
	val1, val0 := "", ""
	switch string(buffer) {
	case "1", "0": val1, val0 = "1", "0"
	case "t", "f": val1, val0 = "t", "f"
	case "T", "F": val1, val0 = "T", "F"
	case "true", "false": val1, val0 = "true", "false"
	case "True", "False": val1, val0 = "True", "False"
	case "TRUE", "FALSE": val1, val0 = "TRUE", "FALSE"
	case "y", "n": val1, val0 = "y", "n"
	case "Y", "N": val1, val0 = "Y", "N"
	case "yes", "no": val1, val0 = "yes", "no"
	case "Yes", "No": val1, val0 = "Yes", "No"
	case "YES", "NO": val1, val0 = "YES", "NO"
	default:
		Warn("Using default handler for bool interface on %s", path)
		//Use 1 and 0 as a last resort
		val1, val0 = "1", "0"
	}
	if data {
		if string(buffer) == val1 {
			Debug("Skipping reset !> %s", path)
			return nil
		}
		dev.BufferWrite(path, val1)
		return nil
	}
	if string(buffer) == val0 {
		Debug("Skipping reset !> %s", path)
		return nil
	}
	dev.BufferWrite(path, val0)
	return nil
}

func (dev *Device) BufferWriteNumber(path string, data interface{}) {
	switch v := data.(type) {
	case float64, float32:
		dev.BufferWrite(path, fmt.Sprintf("%.0F", v))
		return
	}
	dev.BufferWrite(path, fmt.Sprintf("%d", data))
}

func (dev *Device) BufferWrite(path string, data string) {
	if path == "" || data == "" {
		return
	}

	if dev.Buffered == nil {
		dev.Buffered = make([]BufferedWrite, 0)
	}
	found := false
	for i := 0; i < len(dev.Buffered); i++ {
		if dev.Buffered[i].Path == path {
			dev.Buffered[i].Data = data
			found = true
			break
		}
	}
	if !found {
		dev.Buffered = append(dev.Buffered, BufferedWrite{Path: path, Data: data})
	}
}

func (dev *Device) write(path, data string) error {
	if data == "" {
		return nil //Skip empty config options
	}

	buffer, err := ioutil.ReadFile(path)
	if err != nil {
		Warn("Failed to read from path %s: %v", path, err)
	} else {
		if buffer[len(buffer)-1] == '\n' { buffer = buffer[:len(buffer)-1] }
	}

	dataBytes := make([]byte, 0)
	if data == "-" {
		dataBytes = make([]byte, 0) //Write an empty string
	} else {
		dataBytes = []byte(data)
	}

	if string(buffer) == string(dataBytes) {
		Debug("Skipping reset !> %s", path)
		return nil
	}

	if len(dataBytes) > 0 {
		Debug("Writing '%s' > %s", string(dataBytes), path)
	} else {
		Debug("Clearing %s", path)
	}
	/*if len(dataBytes) == 0 || dataBytes[len(dataBytes)-1] != '\n' {
		dataBytes = append(dataBytes, '\n')
	}*/

	//To prevent edge cases with partial applications due to invalid values or inheritance, gracefully log the error and move on
	err = ioutil.WriteFile(path, dataBytes, 0644)
	if err != nil {
		Error("Failed writing '%s' > %s: %v", string(dataBytes), path, err)
	}
	return nil
}
