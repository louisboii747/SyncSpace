//go:build windows

package discovery

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func loadInterfaceDescriptions() map[int]string {
	descriptions := make(map[int]string)
	bufferSize := uint32(15 * 1024)
	flags := uint32(windows.GAA_FLAG_SKIP_UNICAST |
		windows.GAA_FLAG_SKIP_ANYCAST |
		windows.GAA_FLAG_SKIP_MULTICAST |
		windows.GAA_FLAG_SKIP_DNS_SERVER)

	for attempt := 0; attempt < 3; attempt++ {
		buffer := make([]byte, bufferSize)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, first, &bufferSize)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return descriptions
		}
		for adapter := first; adapter != nil; adapter = adapter.Next {
			metadata := strings.TrimSpace(strings.Join([]string{
				windows.UTF16PtrToString(adapter.FriendlyName),
				windows.UTF16PtrToString(adapter.Description),
				windows.BytePtrToString(adapter.AdapterName),
			}, " "))
			if adapter.IfIndex != 0 {
				descriptions[int(adapter.IfIndex)] = metadata
			}
			if adapter.Ipv6IfIndex != 0 {
				descriptions[int(adapter.Ipv6IfIndex)] = metadata
			}
		}
		return descriptions
	}
	return descriptions
}
