//go:build linux && !ppc64 && !ppc64le && !mips && !mipsle && !mips64 && !mips64le

package term

// tcflsh is TCFLSH on x86, arm, arm64, riscv64, s390x and loong64.
const tcflsh = 0x540b
