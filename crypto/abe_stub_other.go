//go:build !windows

package crypto

func SetABEMasterKeyFromHex(_ string) error { return nil }

func GetABEMasterKey() []byte { return nil }
