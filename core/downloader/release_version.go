package downloader

import (
	"fmt"
	"regexp"
	"strings"
)

var releaseVersionRE = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
var numericIdentifierRE = regexp.MustCompile(`^[0-9]+$`)

func releaseVersionParts(s string) ([]string, error) {
	m := releaseVersionRE.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("некорректная semver: %q", s)
	}
	for _, id := range strings.Split(m[4], ".") {
		if numericIdentifierRE.MatchString(id) && len(id) > 1 && id[0] == '0' {
			return nil, fmt.Errorf("некорректная prerelease semver: %q", s)
		}
	}
	return m, nil
}
func compareNumeric(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
func compareReleaseVersions(a, b string) (int, error) {
	aa, e := releaseVersionParts(a)
	if e != nil {
		return 0, e
	}
	bb, e := releaseVersionParts(b)
	if e != nil {
		return 0, e
	}
	for i := 1; i <= 3; i++ {
		if c := compareNumeric(aa[i], bb[i]); c != 0 {
			return c, nil
		}
	}
	if aa[4] == bb[4] {
		return 0, nil
	}
	if aa[4] == "" {
		return 1, nil
	}
	if bb[4] == "" {
		return -1, nil
	}
	ap, bp := strings.Split(aa[4], "."), strings.Split(bb[4], ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		an, bn := numericIdentifierRE.MatchString(ap[i]), numericIdentifierRE.MatchString(bp[i])
		if an && bn {
			if c := compareNumeric(ap[i], bp[i]); c != 0 {
				return c, nil
			}
		} else if an {
			return -1, nil
		} else if bn {
			return 1, nil
		} else if c := strings.Compare(ap[i], bp[i]); c != 0 {
			return c, nil
		}
	}
	if len(ap) < len(bp) {
		return -1, nil
	}
	if len(ap) > len(bp) {
		return 1, nil
	}
	return 0, nil
}
