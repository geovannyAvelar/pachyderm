package postgres

import (
	"debug/elf"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// glibcRequirement is the newest glibc symbol version some ELF file needs.
type glibcRequirement struct {
	Version []int  // e.g. [2 38] for GLIBC_2.38
	File    string // the file needing it, relative to the scanned directory
}

func (r glibcRequirement) String() string {
	parts := make([]string, len(r.Version))
	for i, p := range r.Version {
		parts[i] = fmt.Sprint(p)
	}
	return strings.Join(parts, ".")
}

// requiredGlibc returns the newest GLIBC_x.y symbol version that any ELF
// file under dir needs, or ok=false if none needs glibc.
func requiredGlibc(dir string) (req glibcRequirement, ok bool, err error) {
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		f, err := elf.Open(path)
		if err != nil {
			return nil // not an ELF file
		}
		defer f.Close()

		needs, err := f.DynamicVersionNeeds()
		if err != nil {
			return nil // no version information
		}
		for _, need := range needs {
			for _, dep := range need.Needs {
				v, found := strings.CutPrefix(dep.Dep, "GLIBC_")
				if !found {
					continue
				}
				parts, err := parseVersionParts(v)
				if err != nil {
					continue // e.g. GLIBC_PRIVATE
				}
				if !ok || compareParts(parts, req.Version) > 0 {
					rel, _ := filepath.Rel(dir, path)
					req, ok = glibcRequirement{Version: parts, File: rel}, true
				}
			}
		}
		return nil
	})
	return req, ok, err
}

// systemGlibc returns this system's glibc version, or ok=false if it
// can't tell (no getconf, or a libc other than glibc). It's a variable so
// tests can simulate other systems.
var systemGlibc = func() (version []int, ok bool) {
	out, err := exec.Command("getconf", "GNU_LIBC_VERSION").Output()
	if err != nil {
		return nil, false
	}
	v, found := strings.CutPrefix(strings.TrimSpace(string(out)), "glibc ")
	if !found {
		return nil, false
	}
	parts, err := parseVersionParts(v)
	if err != nil {
		return nil, false
	}
	return parts, true
}

// checkGlibc fails if some ELF file under dir needs a newer glibc than
// this system has. what names those files in the error. A system whose
// glibc version can't be determined passes.
func checkGlibc(dir, what string) error {
	req, ok, err := requiredGlibc(dir)
	if err != nil || !ok {
		return err
	}
	have, ok := systemGlibc()
	if !ok || compareParts(have, req.Version) >= 0 {
		return nil
	}
	return fmt.Errorf(
		"%s needs glibc %s or newer (for %s), but this system has glibc %s. "+
			"Installing it would replace libraries PostgreSQL itself uses and leave it unable to start, so nothing was changed",
		what, req, filepath.ToSlash(req.File), glibcRequirement{Version: have},
	)
}
