package main

import (
	"fmt"
	"io/ioutil"
	"time"
)

type Device struct {
	Paths *Paths //Manifest of paths to device settings
	ProfileBoot string `json:"profile_boot"` //Default profile, used permanently without a profile manager
	ProfileOrder []string //Profile order for inheritance of configurations
	Profiles map[string]*Profile //Manifest of device settings per profile
}

func (dev *Device) Write(path string, data interface{}) error {
	buffer, err := ioutil.ReadFile(path)
	if err != nil {
		Warn("Failed to read from path %s: %v", path, err)
	}
	if buffer[len(buffer)-1] == '\n' { buffer = buffer[:len(buffer)-1] }

	dataBytes := make([]byte, 0)
	switch v := data.(type) {
	case bool:
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
			Warn("No handler for bool interface when writing to path %s (defaulting to 0/1)", path)
			//Use 1 and 0 as a last resort
			val1, val0 = "1", "0"
		}
		if v {
			dataBytes = []byte(val1)
		} else {
			dataBytes = []byte(val0)
		}
	case float64:
		dataBytes = []byte(fmt.Sprintf("%.0F", v))
	case string:
		dataBytes = []byte(v)
	default:
		return fmt.Errorf("unknown data type '%T' when writing to path %s", v, path)
	}

	if string(buffer) == string(dataBytes) {
		//Debug("Skipping reset")
		return nil
	}

	Debug("Writing '%s' > %s", string(dataBytes), path)
	if dataBytes[len(dataBytes)-1] != '\n' {
		dataBytes = append(dataBytes, '\n')
	}
	err = ioutil.WriteFile(path, dataBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write to path %s: %v", path, err)
	}
	return nil
}

func (dev *Device) GetProfile(name string) *Profile {
	return dev.Profiles[name]
}

func (dev *Device) SetProfile(name string) error {
	startTime := time.Now()
	profile := dev.GetProfile(name)
	if profile == nil {
		return fmt.Errorf("profile %s does not exist", name)
	}

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
			max := freq.Max.String()
			if max != "" {
				Debug("> CPUFreq > Max = %s", max)
				maxPath := pathJoin(freqPath, pathFreq.Max)
				Debug(maxPath)
				if err := dev.Write(maxPath, max); err != nil {return err}
			}
			min := freq.Min.String()
			if min != "" {
				minPath := pathJoin(freqPath, pathFreq.Min)
				if debug {
					Debug("> CPUFreq > Min = %s", min)
					Debug(minPath)
				}
				if err := dev.Write(minPath, min); err != nil {return err}
			}
			if freq.Governor != "" {
				governorPath := pathJoin(freqPath, pathFreq.Governor)
				if debug {
					Debug("> CPUFreq > Governor = %s", freq.Governor)
					Debug(governorPath)
				}
				if err := dev.Write(governorPath, freq.Governor); err != nil {return err}
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
					case float64:
						Debug("> %s > %s = %.0F", governorName, arg, v)
					case string:
						Debug("> %s > %s = %s", governorName, arg, v)
					default:
						return fmt.Errorf("governor %s has invalid value type '%T' for arg %s", governorName, v, arg)
					}
					if err := dev.Write(argPath, val); err != nil {return err}
				}
			}
		}
	}

	if profile.CPUSets != nil {
		sets := profile.CPUSets
		setsPath := dev.Paths.Cpusets.Path
		if debug {
			Debug("Loading cpusets")
			Debug(setsPath)
		}
		if sets.Foreground != nil {
			fgPath := pathJoin(setsPath, dev.Paths.Cpusets.Foreground)
			if debug {
				Debug("> CPUSets > Foreground = %s", sets.Foreground)
				Debug(fgPath)
			}
			if sets.Foreground.CPUs != "" {
				if err := dev.Write(fgPath, sets.Foreground.CPUs); err != nil {return err}
			}
			if sets.Foreground.CPUExclusive != "" {
				if err := dev.Write(fgPath, sets.Foreground.CPUExclusive); err != nil {return err}
			}
		}
		if sets.SystemBackground != nil {
			sysbgPath := pathJoin(setsPath, dev.Paths.Cpusets.SystemBackground)
			if debug {
				Debug("> CPUSets > System Background = %s", sets.SystemBackground)
				Debug(sysbgPath)
			}
			if sets.SystemBackground.CPUs != "" {
				if err := dev.Write(sysbgPath, sets.SystemBackground.CPUs); err != nil {return err}
			}
			if sets.SystemBackground.CPUExclusive != "" {
				if err := dev.Write(sysbgPath, sets.SystemBackground.CPUExclusive); err != nil {return err}
			}
		}
		if sets.Background != nil {
			sysbgPath := pathJoin(setsPath, dev.Paths.Cpusets.Background)
			if debug {
				Debug("> CPUSets > Background = %s", sets.Background)
				Debug(sysbgPath)
			}
			if sets.Background.CPUs != "" {
				if err := dev.Write(sysbgPath, sets.Background.CPUs); err != nil {return err}
			}
			if sets.Background.CPUExclusive != "" {
				if err := dev.Write(sysbgPath, sets.Background.CPUExclusive); err != nil {return err}
			}
		}
		if sets.TopApp != nil {
			sysbgPath := pathJoin(setsPath, dev.Paths.Cpusets.TopApp)
			if debug {
				Debug("> CPUSets > Top App = %s", sets.TopApp)
				Debug(sysbgPath)
			}
			if sets.TopApp.CPUs != "" {
				if err := dev.Write(sysbgPath, sets.TopApp.CPUs); err != nil {return err}
			}
			if sets.TopApp.CPUExclusive != "" {
				if err := dev.Write(sysbgPath, sets.TopApp.CPUExclusive); err != nil {return err}
			}
		}
		if sets.Restricted != nil {
			sysbgPath := pathJoin(setsPath, dev.Paths.Cpusets.Restricted)
			if debug {
				Debug("> CPUSets > Restricted = %s", sets.Restricted)
				Debug(sysbgPath)
			}
			if sets.Restricted.CPUs != "" {
				if err := dev.Write(sysbgPath, sets.Restricted.CPUs); err != nil {return err}
			}
			if sets.Restricted.CPUExclusive != "" {
				if err := dev.Write(sysbgPath, sets.Restricted.CPUExclusive); err != nil {return err}
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
				if err := dev.Write(maxPath, max); err != nil {return err}
			}
			min := dvfs.Min.String()
			if min != "" {
				minPath := pathJoin(gpuPath, dev.Paths.GPU.DVFS.Min)
				if debug {
					Debug("> GPU > DVFS > Min = %s", min)
					Debug(minPath)
				}
				if err := dev.Write(minPath, min); err != nil {return err}
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
				if err := dev.Write(clockPath, clock); err != nil {return err}
			}
			load := hs.Load.String()
			if load != "" {
				loadPath := pathJoin(gpuPath, dev.Paths.GPU.Highspeed.Load)
				if debug {
					Debug("> GPU > Highspeed > Load = %s", load)
					Debug(loadPath)
				}
				if err := dev.Write(loadPath, load); err != nil {return err}
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
		if err := dev.Write(dynamicHotplugPath, krnl.DynamicHotplug); err != nil {return err}
		powerEfficientPath := dev.Paths.Kernel.PowerEfficient
		if debug {
			Debug("> Kernel > Power Efficient = %t", krnl.PowerEfficient)
			Debug(powerEfficientPath)
		}
		if err := dev.Write(powerEfficientPath, krnl.PowerEfficient); err != nil {return err}
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
			if err := dev.Write(boostPath, hmp.Boost); err != nil {return err}
			semiboostPath := pathJoin(hmpPath, hmpPaths.Semiboost)
			if debug {
				Debug("> Kernel > HMP > Semiboost = %t", hmp.Semiboost)
				Debug(semiboostPath)
			}
			if err := dev.Write(semiboostPath, hmp.Semiboost); err != nil {return err}
			activeDownMigrationPath := pathJoin(hmpPath, hmpPaths.ActiveDownMigration)
			if debug {
				Debug("> Kernel > HMP > Active Down Migration = %t", hmp.ActiveDownMigration)
				Debug(activeDownMigrationPath)
			}
			if err := dev.Write(activeDownMigrationPath, hmp.ActiveDownMigration); err != nil {return err}
			aggressiveUpMigrationPath := pathJoin(hmpPath, hmpPaths.AggressiveUpMigration)
			if debug {
				Debug("> Kernel > HMP > Aggressive Up Migration = %t", hmp.AggressiveUpMigration)
				Debug(aggressiveUpMigrationPath)
			}
			if err := dev.Write(aggressiveUpMigrationPath, hmp.AggressiveUpMigration); err != nil {return err}
			if hmp.Threshold != nil {
				thld := hmp.Threshold
				down := thld.Down.String()
				if down != "" {
					downPath := pathJoin(hmpPath, hmpPaths.Threshold.Down)
					if debug {
						Debug("> Kernel > HMP > Threshold > Down = %s", down)
						Debug(downPath)
					}
					if err := dev.Write(downPath, down); err != nil {return err}
				}
				up := thld.Up.String()
				if up != "" {
					upPath := pathJoin(hmpPath, hmpPaths.Threshold.Up)
					if debug {
						Debug("> Kernel > HMP > Threshold > Up = %s", up)
						Debug(upPath)
					}
					if err := dev.Write(upPath, up); err != nil {return err}
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
					if err := dev.Write(downPath, down); err != nil {return err}
				}
				up := thld.Up.String()
				if up != "" {
					upPath := pathJoin(hmpPath, hmpPaths.SbThreshold.Up)
					if debug {
						Debug("> Kernel > HMP > Semiboost Threshold > Up = %s", up)
						Debug(upPath)
					}
					if err := dev.Write(upPath, up); err != nil {return err}
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
		if err := dev.Write(enabledPath, ipa.Enabled); err != nil {return err}
		if ipa.Enabled {
			controlTemp := ipa.ControlTemp.String()
			if controlTemp != "" {
				ctPath := pathJoin(ipaPath, ipaPaths.ControlTemp)
				if debug {
					Debug("> IPA > Control Temp = %s", controlTemp)
					Debug(ctPath)
				}
				if err := dev.Write(ctPath, controlTemp); err != nil {return err}
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
			if err := dev.Write(headPath, ib.Head); err != nil {return err}
		}
		if ib.Tail != "" {
			tailPath := ibPaths.Tail
			if debug {
				Debug("> Input Booster > Tail = %s", ib.Tail)
				Debug(tailPath)
			}
			if err := dev.Write(tailPath, ib.Tail); err != nil {return err}
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
		if err := dev.Write(enabledPath, slowPaths.Enabled); err != nil {return err}
		if slow.Enabled {
			enforcedPath := slowPaths.Enforced
			if debug {
				Debug("> sec_slow > Enforced = %t", slow.Enforced)
				Debug(enforcedPath)
			}
			if err := dev.Write(enforcedPath, slow.Enforced); err != nil {return err}
			timerRate := slow.TimerRate.String()
			if timerRate != "" {
				timerRatePath := slowPaths.TimerRate
				if debug {
					Debug("> sec_slow > Timer Rate = %s", timerRate)
					Debug(timerRatePath)
				}
				if err := dev.Write(timerRatePath, timerRate); err != nil {return err}
			}
		}
	}

	deltaTime := time.Now().Sub(startTime).Milliseconds()
	Info("PowerPulse finished applying %s in %dms", name, deltaTime)
	return nil
}
