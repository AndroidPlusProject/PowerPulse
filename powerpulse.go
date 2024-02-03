package main

import (
	"C"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

var (
	device *Device = nil
	manifests = []string{
		"./powerpulse.json",
		"/data/local/tmp/powerpulse.json",
		"/vendor/etc/powerpulse.json",
		"/system/vendor/etc/powerpulse.json",
		"/system/etc/powerpulse.json",
		"/etc/powerpulse.json",
	}
	profileNow = ""
	profileLast = ""
	debug = true
	verbose = true
	booted = false
)

//export PowerPulse_SetProfile
func PowerPulse_SetProfile(profile *C.char) {
	go setProfile(C.GoString(profile))
}
func setProfile(profile string) {
	PowerPulse_Init()
	if device == nil {
		return
	}
	if err := device.SetProfile(profile); err != nil {
		Error("Error applying profile %s: %v", profile, err)
		return
	}
	profileLast = profileNow
	profileNow = profile
}

//export PowerPulse_ResetProfile
func PowerPulse_ResetProfile() {
	go resetProfile()
}
func resetProfile() {
	PowerPulse_Init()
	if device == nil {
		return
	}
	if profileLast != "" {
		if err := device.SetProfile(profileLast); err != nil {
			Error("Error resetting to profile %s: %v", profileLast, err)
			return
		}
		profileTmp := profileNow
		profileNow = profileLast
		profileLast = profileTmp
	}
}

//export PowerPulse_SetInteractive
func PowerPulse_SetInteractive(interactive bool) {
	Debug("Interactive:", interactive)
}

//export PowerPulse_SetPowerHint
func PowerPulse_SetPowerHint(hint, data int32) {
	Debug("PowerHint:", hint, data)
}

//export PowerPulse_SetFeature
func PowerPulse_SetFeature(feature int32, activate bool) {
	Debug("SetFeature:", feature)
}

//export PowerPulse_GetFeature
func PowerPulse_GetFeature(feature uint32) int32 {
	Debug("GetFeature:", feature)
	return 0
}

//export PowerPulse_Init
func PowerPulse_Init() {
	if booted {
		return
	}
	booted = true
	startTime := time.Now()

	Info("Need to boot PowerPulse first, just a blip...")
	PowerPulse_ReloadConfig()

	deltaTime := time.Now().Sub(startTime).Milliseconds()
	Info("PowerPulse finished init in %dms", deltaTime)
}

//export PowerPulse_ReloadConfig
func PowerPulse_ReloadConfig() {
	deviceJSON := make([]byte, 0)
	for i := 0; i < len(manifests); i++ {
		tmpJSON, err := ioutil.ReadFile(manifests[i])
		if err == nil && len(tmpJSON) > 0 {
			Info("Found manifest at %s", manifests[i])
			deviceJSON = tmpJSON
			break
		}
	}

	device = &Device{}
	if err := json.Unmarshal(deviceJSON, device); err != nil {
		Error("Error parsing manifest: %v", err)
		return
	}

	if device.Paths == nil {
		device.Paths = &Paths{}
	}
	if err := device.Paths.Init(); err != nil {
		Error("Error parsing paths from device manifest: %v", err)
		return
	}

	pathsJSON, err := json.Marshal(device.Paths)
	if err != nil {
		Debug("DEBUG: Error marshalling paths for print: %v", err)
		return
	}
	Debug(string(pathsJSON))

	if len(device.Profiles) < 1 {
		Error("Error reading device manifest: No profiles were found")
		return
	}

	for profileName := range device.Profiles {
		adjustedName := strings.ReplaceAll(strings.ToLower(profileName), " ", "_")
		if adjustedName != profileName {
			device.Profiles[adjustedName] = device.Profiles[profileName]
			delete(device.Profiles, profileName)
			Debug("Found profile %s as %s", profileName, adjustedName)
		} else {
			Debug("Found profile %s", adjustedName)
		}
	}

	if device.ProfileBoot != "" {
		device.ProfileBoot = strings.ReplaceAll(strings.ToLower(device.ProfileBoot), " ", "_")
	}

	if profileNow == "" {
		if device.ProfileBoot != "" {
			profileNow = strings.ReplaceAll(strings.ToLower(device.ProfileBoot), " ", "_")
		}
		if device.Paths.PowerPulse != nil && device.Paths.PowerPulse.Profile != "" {
			profileBytes, err := ioutil.ReadFile(device.Paths.PowerPulse.Profile)
			if err == nil && len(profileBytes) > 0 {
				profileNow = strings.ReplaceAll(strings.ToLower(device.Paths.PowerPulse.Profile), " ", "_")
			}
		}
	} else {
		profileNow = strings.ReplaceAll(strings.ToLower(profileNow), " ", "_")
	}

	if device.ProfileOrder == nil || len(device.ProfileOrder) == 0 {
		Debug("No profile order was specified")
		//Try to add any recognizable profiles
		po := make([]string, 0)
		if p := device.GetProfile("battery_saver"); p != nil {
			Debug("Found profile battery_saver")
			po = append(po, "battery_saver")
		}
		if p := device.GetProfile("efficiency"); p != nil {
			Debug("Found profile efficiency")
			po = append(po, "efficiency")
		}
		if p := device.GetProfile("balanced"); p != nil {
			Debug("Found profile balanced")
			po = append(po, "balanced")
		}
		if p := device.GetProfile("quick"); p != nil {
			Debug("Found profile quick")
			po = append(po, "quick")
		}
		if p := device.GetProfile("performance"); p != nil {
			Debug("Found profile performance")
			po = append(po, "performance")
		}
		found := false
		for i := 0; i < len(po); i++ {
			if po[i] == profileNow {
				found = true
				break
			}
		}
		if !found {
			//Start with the configured boot profile, in case we inherit special settings (better to be safe than sorry!)
			po = append([]string{profileNow}, po...)
		}
		device.ProfileOrder = po
	}
	if len(device.ProfileOrder) == 0 {
		Error("Error reading device manifest!")
		Debug("No identifiable boot profile, please set your profile order and/or your boot profile!")
		return
	}
	Debug("Profile order: %s", device.ProfileOrder)
}

func main() {
	debug = false
	verbose = false
	pflag.StringArrayVarP(&manifests, "manifest", "m", manifests, "path to device manifest(s),comma-separated")
	pflag.StringVarP(&profileNow, "profile", "p", profileNow, "profile override")
	pflag.BoolVarP(&debug, "debug", "d", debug, "debug mode")
	pflag.BoolVarP(&verbose, "verbose", "v", verbose, "verbose mode")
	pflag.Parse()
	PowerPulse_Init()

	Info("Stargazing for desired profile")
	found := false
	for profileName := range device.Profiles {
		if profileNow == profileName {
			found = true
			break
		}
	}
	if !found {
		profileNow = device.ProfileOrder[len(device.ProfileOrder)-1]
	}

	Info("Applying profile %s", profileNow)
	setProfile(profileNow)
}
