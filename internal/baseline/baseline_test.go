package baseline

import (
	"os"
	"path/filepath"
	"testing"

	cpb "github.com/google/capslock/proto"
	"google.golang.org/protobuf/proto"
)

func sample() *cpb.CapabilityInfoList {
	return &cpb.CapabilityInfoList{
		CapabilityInfo: []*cpb.CapabilityInfo{
			{
				PackageDir:     proto.String("github.com/example/app"),
				PackageName:    proto.String("app"),
				CapabilityName: proto.String("FILES"),
				Path: []*cpb.Function{
					{Name: proto.String("app.Read"), Package: proto.String("github.com/example/app")},
					{Name: proto.String("os.ReadFile"), Package: proto.String("os")},
				},
			},
			{
				PackageDir:     proto.String("github.com/example/app"),
				PackageName:    proto.String("app"),
				CapabilityName: proto.String("NETWORK"),
			},
		},
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "baseline.json")
	in := sample()
	if err := Write(path, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(out.CapabilityInfo) != len(in.CapabilityInfo) {
		t.Fatalf("len = %d, want %d", len(out.CapabilityInfo), len(in.CapabilityInfo))
	}
	for i := range in.CapabilityInfo {
		if out.CapabilityInfo[i].GetPackageDir() != in.CapabilityInfo[i].GetPackageDir() {
			t.Errorf("[%d] PackageDir = %q, want %q", i, out.CapabilityInfo[i].GetPackageDir(), in.CapabilityInfo[i].GetPackageDir())
		}
		if out.CapabilityInfo[i].GetCapabilityName() != in.CapabilityInfo[i].GetCapabilityName() {
			t.Errorf("[%d] CapabilityName = %q, want %q", i, out.CapabilityInfo[i].GetCapabilityName(), in.CapabilityInfo[i].GetCapabilityName())
		}
	}
	if len(out.CapabilityInfo[0].Path) != 2 {
		t.Errorf("Path len = %d, want 2", len(out.CapabilityInfo[0].Path))
	}
}

func TestReadMissing(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "absent.json"))
	if !IsNotExist(err) {
		t.Errorf("Read missing: IsNotExist = false, err = %v", err)
	}
}

func TestReadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Error("Read invalid: want error, got nil")
	}
}

func TestWriteCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "baseline.json")
	if err := Write(path, sample()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
