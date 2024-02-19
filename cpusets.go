package main

func (dev *Device) setCpusets(profile *Profile) error {
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
		dev.SyncProfile()

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
		dev.SyncProfile()

		//Finally, set up any new CPU exclusives
		for setName, set := range sets {
			if set.CPUExclusive != nil {
				setPath := pathJoin(setsPath, setName)
				if debug {
					Debug("> CPUSets > %s > Exclusive CPU = %t", setName, set.CPUExclusive)
					Debug(setPath)
				}
				exclusivePath := pathJoin(setPath, dev.Paths.Cpusets.Sets[setName].CPUExclusive)
				if err := dev.BufferWriteBool(exclusivePath, *set.CPUExclusive); err != nil {return err}
			}
		}
		dev.SyncProfile()
	}
	return nil
}