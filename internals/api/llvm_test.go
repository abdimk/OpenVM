package api

import "testing"

var assets2311 = []GitHubAsset{
	{Name: "clang+llvm-23.1.1-aarch64-pc-windows-msvc.tar.xz", Size: 1},
	{Name: "clang+llvm-23.1.1-aarch64-pc-windows-msvc.tar.xz.jsonl", Size: 1},
	{Name: "clang+llvm-23.1.1-aarch64-pc-windows-msvc.tar.zst", Size: 1},
	{Name: "clang+llvm-23.1.1-aarch64-pc-windows-msvc.tar.zst.jsonl", Size: 1},
	{Name: "clang+llvm-23.1.1-x86_64-pc-windows-msvc.tar.xz", Size: 1},
	{Name: "clang+llvm-23.1.1-x86_64-pc-windows-msvc.tar.xz.jsonl", Size: 1},
	{Name: "clang+llvm-23.1.1-x86_64-pc-windows-msvc.tar.zst", Size: 1},
	{Name: "clang+llvm-23.1.1-x86_64-pc-windows-msvc.tar.zst.jsonl", Size: 1},
	{Name: "FLANG-23.1.1-Linux-X64.tar.xz", Size: 1},
	{Name: "LLVM-23.1.1-Linux-ARM64.tar.xz", Size: 1},
	{Name: "LLVM-23.1.1-Linux-ARM64.tar.xz.jsonl", Size: 1},
	{Name: "LLVM-23.1.1-Linux-ARM64.tar.zst", Size: 1},
	{Name: "LLVM-23.1.1-Linux-ARM64.tar.zst.jsonl", Size: 1},
	{Name: "LLVM-23.1.1-Linux-X64.tar.xz", Size: 1},
	{Name: "LLVM-23.1.1-Linux-X64.tar.xz.jsonl", Size: 1},
	{Name: "LLVM-23.1.1-Linux-X64.tar.zst", Size: 1},
	{Name: "LLVM-23.1.1-Linux-X64.tar.zst.jsonl", Size: 1},
	{Name: "LLVM-23.1.1-macOS-ARM64.tar.xz", Size: 1},
	{Name: "LLVM-23.1.1-macOS-ARM64.tar.xz.jsonl", Size: 1},
	{Name: "LLVM-23.1.1-macOS-ARM64.tar.zst", Size: 1},
	{Name: "LLVM-23.1.1-macOS-ARM64.tar.zst.jsonl", Size: 1},
	{Name: "LLVM-23.1.1-win64.msi", Size: 1},
	{Name: "LLVM-23.1.1-win64.msi.jsonl", Size: 1},
	{Name: "LLVM-23.1.1-woa64.msi", Size: 1},
	{Name: "LLVM-23.1.1-woa64.msi.jsonl", Size: 1},
	{Name: "clang-tools-extra_doxygen-23.1.1.tar.xz", Size: 1},
	{Name: "clang_doxygen-23.1.1.tar.xz", Size: 1},
	{Name: "llvm-project-23.1.1.src.tar.xz", Size: 1},
	{Name: "llvm-project-23.1.1.src.tar.xz.sig", Size: 1},
	{Name: "llvm_doxygen-23.1.1.tar.xz", Size: 1},
	{Name: "llvm_man_pages-23.1.1.tar.xz", Size: 1},
	{Name: "test-suite-23.1.1.src.tar.xz", Size: 1},
}

func TestLLVMAssetForWindowsAMD64(t *testing.T) {
	file, ok := llvmAssetFor("23.1.1", assets2311, "windows", "amd64")
	if !ok {
		t.Fatal("expected a Windows amd64 asset to match")
	}
	if want := "clang+llvm-23.1.1-x86_64-pc-windows-msvc.tar.zst"; file.Filename != want {
		t.Errorf("filename = %q, want %q (zstd preferred over xz)", file.Filename, want)
	}
	if file.OS != "windows" || file.Arch != "amd64" {
		t.Errorf("got %s/%s, want windows/amd64", file.OS, file.Arch)
	}
}

func TestLLVMAssetForWindowsARM64(t *testing.T) {
	file, ok := llvmAssetFor("23.1.1", assets2311, "windows", "arm64")
	if !ok {
		t.Fatal("expected a Windows arm64 asset to match")
	}
	if want := "clang+llvm-23.1.1-aarch64-pc-windows-msvc.tar.zst"; file.Filename != want {
		t.Errorf("filename = %q, want %q", file.Filename, want)
	}
}

func TestLLVMAssetForLinuxAMD64(t *testing.T) {
	file, ok := llvmAssetFor("23.1.1", assets2311, "linux", "amd64")
	if !ok {
		t.Fatal("expected a Linux amd64 asset to match")
	}
	if want := "LLVM-23.1.1-Linux-X64.tar.zst"; file.Filename != want {
		t.Errorf("filename = %q, want %q", file.Filename, want)
	}
}

func TestLLVMAssetForLinuxARM64(t *testing.T) {
	file, ok := llvmAssetFor("23.1.1", assets2311, "linux", "arm64")
	if !ok {
		t.Fatal("expected a Linux arm64 asset to match")
	}
	if want := "LLVM-23.1.1-Linux-ARM64.tar.zst"; file.Filename != want {
		t.Errorf("filename = %q, want %q", file.Filename, want)
	}
}

func TestLLVMAssetForDarwinARM64(t *testing.T) {
	file, ok := llvmAssetFor("23.1.1", assets2311, "darwin", "arm64")
	if !ok {
		t.Fatal("expected a macOS arm64 asset to match")
	}
	if want := "LLVM-23.1.1-macOS-ARM64.tar.zst"; file.Filename != want {
		t.Errorf("filename = %q, want %q", file.Filename, want)
	}
}

func TestLLVMAssetForDarwinAMD64(t *testing.T) {

	if _, ok := llvmAssetFor("23.1.1", assets2311, "darwin", "amd64"); ok {
		t.Fatal("expected no Darwin amd64 asset in LLVM 23.1.1")
	}
}

func TestLLVMAssetIgnoresNoise(t *testing.T) {
	for _, name := range []string{
		"llvm-project-23.1.1.src.tar.xz",
		"llvm-project-23.1.1.src.tar.xz.sig",
		"LLVM-23.1.1-win64.msi",
		"clang_doxygen-23.1.1.tar.xz",
		"llvm_man_pages-23.1.1.tar.xz",
		"clang+llvm-23.1.1-x86_64-pc-windows-msvc.tar.xz.jsonl",
		"test-suite-23.1.1.src.tar.xz",
	} {
		if ok := llvmMatches(name, "windows", "amd64"); ok {
			t.Errorf("llvmMatches(%q) should be false", name)
		}
	}
}

func TestLLVMMatchesOlderNaming(t *testing.T) {

	cases := []struct {
		name, os, arch string
		want           bool
	}{
		{"LLVM-18.1.8-Windows-X64.zip", "windows", "amd64", true},
		{"LLVM-18.1.8-Linux-X64.tar.xz", "linux", "amd64", true},
		{"LLVM-18.1.8-Linux-AArch64.tar.xz", "linux", "arm64", true},
		{"LLVM-16.0.0-Darwin-ARM64.tar.gz", "darwin", "arm64", true},
		{"clang+llvm-16.0.0-x86_64-pc-windows-msvc.tar.xz", "windows", "amd64", true},
		{"clang+llvm-17.0.6-x86_64-linux-gnu-ubuntu-22.04.tar.xz", "linux", "amd64", true},
		{"clang+llvm-16.0.0-aarch64-linux-gnu.tar.xz", "linux", "arm64", true},
		{"LLVM-18.1.8-Windows-ARM64.zip", "windows", "arm64", true},
		{"LLVM-18.1.8-Linux-X64.tar.xz", "windows", "amd64", false},
		{"LLVM-18.1.8-Windows-X64.zip", "linux", "amd64", false},
	}
	for _, c := range cases {
		if got := llvmMatches(c.name, c.os, c.arch); got != c.want {
			t.Errorf("llvmMatches(%q, %s, %s) = %v, want %v", c.name, c.os, c.arch, got, c.want)
		}
	}
}
