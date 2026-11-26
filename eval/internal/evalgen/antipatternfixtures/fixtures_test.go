package antipatternfixtures_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const profileIdentity = "1111111111111111111111111111111111111111111111111111111111111111"

// profileWith is a profile in the committed schema's shape, with the given
// shapes and constructor counts, labelled "secret-label" so a row can check
// that the label reaches no fixture.
func profileWith(nesting, cognitive, branchDensity map[string]int, doesWork int) []byte {
	GinkgoHelper()
	profile := map[string]any{
		"schema_version": "1", "identity": profileIdentity, "label": "secret-label",
		"sha": "0123456789abcdef0123456789abcdef01234567", "licence": "MIT", "extractor_version": "0.2.0",
		"roles": map[string]any{},
		"dimensions": map[string]any{
			"layers": map[string]int{}, "entry": map[string]int{"http_handler": 0, "method": 0, "function": 1},
			"depth": map[string]int{}, "trap": nil, "field": nil, "direction": nil,
		},
		"shapes": map[string]any{
			"nesting": nesting, "cognitive": cognitive, "branch_density": branchDensity,
			"constructor": map[string]int{"assigns_only": 0, "does_work": doesWork},
		},
		"not_covered": []any{},
	}
	raw, err := json.Marshal(profile)
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// everyShape is a profile that observed every shape a template reads.
func everyShape() []byte {
	return profileWith(map[string]int{"2": 9, "4": 3, "5": 1}, map[string]int{"3": 4, "19": 2}, map[string]int{"13": 2}, 1)
}

func acceptAll([]byte) error { return nil }

type expectFile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Source           string `json:"source"`
	Seed             string `json:"seed"`
	Status           string `json:"status"`
	Note             string `json:"note"`
	ExpectIncomplete bool   `json:"expect_incomplete"`
	Expect           struct {
		Bad  json.RawMessage `json:"bad"`
		Good json.RawMessage `json:"good"`
	} `json:"expect"`
}

func readExpect(out, id string) expectFile {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(out, id, "expect.json"))
	Expect(err).NotTo(HaveOccurred())
	var expect expectFile
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	Expect(decoder.Decode(&expect)).To(Succeed(), "expect.json uses only #871's fields")
	return expect
}

type finding struct {
	RuleID  string `json:"rule_id"`
	Path    string `json:"path"`
	Subject string `json:"subject"`
}

func badFindings(expect expectFile) []finding {
	GinkgoHelper()
	var findings []finding
	Expect(json.Unmarshal(expect.Expect.Bad, &findings)).To(Succeed(), string(expect.Expect.Bad))
	return findings
}

// tree reads every file under dir, keyed by its slash path.
func tree(dir string) map[string]string {
	GinkgoHelper()
	files := map[string]string{}
	Expect(filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(raw)
		return err
	})).To(Succeed())
	return files
}

func ids(dir string) []string {
	GinkgoHelper()
	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred())
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func exitOf(err error) int {
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return -1
}
