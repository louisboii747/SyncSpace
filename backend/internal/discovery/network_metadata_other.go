//go:build !windows

package discovery

func loadInterfaceDescriptions() map[int]string {
	return nil
}
