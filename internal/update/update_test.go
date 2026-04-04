package update

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestParseSemver verifies semver parsing with table-driven cases.
//
// **Validates: Requirements 5.2, 5.4**
func TestParseSemver(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantMaj, wantMin int
		wantPat          int
		wantErr          bool
	}{
		{"basic", "1.0.4", 1, 0, 4, false},
		{"with v prefix", "v1.0.4", 1, 0, 4, false},
		{"zeros", "0.0.0", 0, 0, 0, false},
		{"large numbers", "10.20.30", 10, 20, 30, false},
		{"pre-release suffix stripped", "v1.0.3-1-g87fe3a1-dirty", 1, 0, 3, false},
		{"bad string", "bad", 0, 0, 0, true},
		{"empty string", "", 0, 0, 0, true},
		{"two parts", "1.0", 0, 0, 0, true},
		{"non-numeric major", "a.0.1", 0, 0, 0, true},
		{"non-numeric minor", "1.b.1", 0, 0, 0, true},
		{"non-numeric patch", "1.0.c", 0, 0, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			maj, min, pat, err := ParseSemver(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseSemver(%q) expected error, got (%d,%d,%d,nil)", tc.input, maj, min, pat)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSemver(%q) unexpected error: %v", tc.input, err)
			}
			if maj != tc.wantMaj || min != tc.wantMin || pat != tc.wantPat {
				t.Fatalf("ParseSemver(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tc.input, maj, min, pat, tc.wantMaj, tc.wantMin, tc.wantPat)
			}
		})
	}
}

// TestIsNewer verifies semver comparison with table-driven cases.
//
// **Validates: Requirements 5.2, 5.4**
func TestIsNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.4", "1.0.3", true},
		{"1.0.3", "1.0.4", false},
		{"1.0.3", "1.0.3", false},
		{"2.0.0", "1.9.9", true},
		{"1.1.0", "1.0.9", true},
		{"0.0.1", "0.0.0", true},
		{"v1.0.4", "v1.0.3", true},
		{"v1.0.3", "1.0.4", false},
		// Invalid inputs return false.
		{"bad", "1.0.0", false},
		{"1.0.0", "bad", false},
	}

	for _, tc := range tests {
		name := fmt.Sprintf("%s_vs_%s", tc.a, tc.b)
		t.Run(name, func(t *testing.T) {
			got := IsNewer(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("IsNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestPropertyP5_2_SemverComparisonIsCorrect verifies that semver comparison
// is correct across randomly generated version triples.
//
// **Property P5.2: Semver comparison is correct**
// **Validates: Requirements 5.2, 5.4**
func TestPropertyP5_2_SemverComparisonIsCorrect(t *testing.T) {
	semverComponent := rapid.IntRange(0, 100)

	rapid.Check(t, func(rt *rapid.T) {
		aMaj := semverComponent.Draw(rt, "aMaj")
		aMin := semverComponent.Draw(rt, "aMin")
		aPat := semverComponent.Draw(rt, "aPat")
		bMaj := semverComponent.Draw(rt, "bMaj")
		bMin := semverComponent.Draw(rt, "bMin")
		bPat := semverComponent.Draw(rt, "bPat")

		a := fmt.Sprintf("%d.%d.%d", aMaj, aMin, aPat)
		b := fmt.Sprintf("%d.%d.%d", bMaj, bMin, bPat)

		// Property 1: ParseSemver round-trips correctly for valid versions.
		gotMaj, gotMin, gotPat, err := ParseSemver(a)
		if err != nil {
			rt.Fatalf("ParseSemver(%q) failed: %v", a, err)
		}
		if gotMaj != aMaj || gotMin != aMin || gotPat != aPat {
			rt.Fatalf("ParseSemver(%q) = (%d,%d,%d), want (%d,%d,%d)",
				a, gotMaj, gotMin, gotPat, aMaj, aMin, aPat)
		}

		// Property 2: IsNewer is consistent with integer comparison.
		aIsNewer := aMaj > bMaj ||
			(aMaj == bMaj && aMin > bMin) ||
			(aMaj == bMaj && aMin == bMin && aPat > bPat)

		if IsNewer(a, b) != aIsNewer {
			rt.Fatalf("IsNewer(%q, %q) = %v, expected %v", a, b, IsNewer(a, b), aIsNewer)
		}

		// Property 3: Irreflexivity — no version is newer than itself.
		if IsNewer(a, a) {
			rt.Fatalf("IsNewer(%q, %q) should be false (irreflexive)", a, a)
		}

		// Property 4: Asymmetry — if a > b then b is not > a.
		if IsNewer(a, b) && IsNewer(b, a) {
			rt.Fatalf("IsNewer is not asymmetric: both (%q > %q) and (%q > %q)", a, b, b, a)
		}

		// Property 5: v-prefix is transparent.
		vA := "v" + a
		if IsNewer(a, b) != IsNewer(vA, b) {
			rt.Fatalf("v-prefix changed result: IsNewer(%q,%q)=%v vs IsNewer(%q,%q)=%v",
				a, b, IsNewer(a, b), vA, b, IsNewer(vA, b))
		}
	})
}

// TestBuildDownloadURL verifies download URL construction with table-driven cases.
//
// **Validates: Requirement 5.6**
func TestBuildDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		version string
		goos    string
		goarch  string
		want    string
	}{
		{
			name:    "darwin/arm64",
			version: "1.0.4",
			goos:    "darwin",
			goarch:  "arm64",
			want:    "https://github.com/npozs77/VocabGen/releases/download/v1.0.4/vocabgen_darwin_arm64.tar.gz",
		},
		{
			name:    "linux/amd64",
			version: "1.0.4",
			goos:    "linux",
			goarch:  "amd64",
			want:    "https://github.com/npozs77/VocabGen/releases/download/v1.0.4/vocabgen_linux_amd64.tar.gz",
		},
		{
			name:    "windows/amd64",
			version: "1.0.4",
			goos:    "windows",
			goarch:  "amd64",
			want:    "https://github.com/npozs77/VocabGen/releases/download/v1.0.4/vocabgen_windows_amd64.zip",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildDownloadURL(tc.version, tc.goos, tc.goarch)
			if got != tc.want {
				t.Fatalf("BuildDownloadURL(%q, %q, %q) = %q, want %q",
					tc.version, tc.goos, tc.goarch, got, tc.want)
			}
		})
	}
}

// TestPropertyP5_3_DownloadLinkMatchesOSAndArch verifies that the download URL
// always contains the correct OS, architecture, and extension for any valid input.
//
// **Property P5.3: Download link matches the running OS and architecture**
// **Validates: Requirement 5.6**
func TestPropertyP5_3_DownloadLinkMatchesOSAndArch(t *testing.T) {
	osGen := rapid.SampledFrom([]string{"darwin", "linux", "windows", "freebsd"})
	archGen := rapid.SampledFrom([]string{"amd64", "arm64", "386"})
	semverComponent := rapid.IntRange(0, 100)

	rapid.Check(t, func(rt *rapid.T) {
		maj := semverComponent.Draw(rt, "major")
		min := semverComponent.Draw(rt, "minor")
		pat := semverComponent.Draw(rt, "patch")
		version := fmt.Sprintf("%d.%d.%d", maj, min, pat)

		goos := osGen.Draw(rt, "goos")
		goarch := archGen.Draw(rt, "goarch")

		url := BuildDownloadURL(version, goos, goarch)

		// Property 1: URL starts with the download base and includes the version tag.
		wantPrefix := fmt.Sprintf("%s/v%s/", DownloadURLBase, version)
		if !strings.HasPrefix(url, wantPrefix) {
			rt.Fatalf("URL %q does not start with %q", url, wantPrefix)
		}

		// Property 2: URL contains the OS and architecture in the filename.
		wantFragment := fmt.Sprintf("vocabgen_%s_%s.", goos, goarch)
		if !strings.Contains(url, wantFragment) {
			rt.Fatalf("URL %q does not contain %q", url, wantFragment)
		}

		// Property 3: Windows gets .zip, everything else gets .tar.gz.
		if goos == "windows" {
			if !strings.HasSuffix(url, ".zip") {
				rt.Fatalf("Windows URL %q should end with .zip", url)
			}
		} else {
			if !strings.HasSuffix(url, ".tar.gz") {
				rt.Fatalf("Non-windows URL %q should end with .tar.gz", url)
			}
		}
	})
}
