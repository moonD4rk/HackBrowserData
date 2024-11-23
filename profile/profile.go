package profile

import (
	"github.com/moond4rk/hackbrowserdata/types2"
)

type Profiles map[string]*Profile

func NewProfiles() Profiles {
	return make(Profiles)
}

func (profiles Profiles) GetOrCreateProfile(profileName string) *Profile {
	profile, ok := profiles[profileName]
	if !ok {
		profile = NewProfile(profileName)
		profiles[profileName] = profile
	}
	return profile
}

func (profiles Profiles) SetDataTypePath(profileName string, dataType types2.DataType, path string) {
	profile := profiles.GetOrCreateProfile(profileName)
	profile.AddPath(dataType, path)
}

func (profiles Profiles) SetMasterKey(path string) {
	for _, profile := range profiles {
		profile.MasterKeyPath = path
	}
}

func (profiles Profiles) AssignMasterKey() {
	for _, profile := range profiles {
		keyPath, ok := profile.DataFilePath[types2.MasterKey]
		if ok && len(keyPath) > 0 {
			profile.MasterKeyPath = keyPath[0]
			delete(profile.DataFilePath, types2.MasterKey)
		}
	}
}

type Profile struct {
	Name        string
	BrowserType types2.BrowserType
	// MasterKeyPath is the path to the master key file.
	// chromium - "Local State" is shared by all profiles
	// firefox - "key4.db" is unique per profile
	MasterKeyPath string
	DataFilePath  map[types2.DataType][]string
}

func NewProfile(profileName string) *Profile {
	return &Profile{
		Name:         profileName,
		DataFilePath: make(map[types2.DataType][]string),
	}
}

func (p *Profile) AddPath(dataType types2.DataType, path string) {
	p.DataFilePath[dataType] = append(p.DataFilePath[dataType], path)
}
