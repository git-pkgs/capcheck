package fixture

import (
	"net"
	"os"
	"os/exec"
)

func ReadFile(p string) ([]byte, error) {
	return os.ReadFile(p)
}

func Dial(addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}

func Run(name string) error {
	return exec.Command(name).Run()
}
