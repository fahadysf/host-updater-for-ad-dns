//go:build !linux

package main

func getDefaultInterfaceAddresses() (IPAddrs, error) {
	return getDefaultInterfaceAddressesFallback()
}
