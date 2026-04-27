package baseline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cpb "github.com/google/capslock/proto"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	filePerm = 0o644
	dirPerm  = 0o755
)

func Read(path string) (*cpb.CapabilityInfoList, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cil := new(cpb.CapabilityInfoList)
	if err := protojson.Unmarshal(b, cil); err != nil {
		return nil, fmt.Errorf("parsing baseline %s: %w", path, err)
	}
	return cil, nil
}

func Write(path string, cil *cpb.CapabilityInfoList) error {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(cil)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return err
		}
	}
	return os.WriteFile(path, b, filePerm)
}

func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
