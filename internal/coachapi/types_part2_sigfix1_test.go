package coachapi

import (
	"encoding/json"
	"testing"
)

type sigTestReportMarshalMatchesGoldenFileS158059557 struct {
	err error
	t   *testing.
		T
}

func (sigRecv *sigTestReportMarshalMatchesGoldenFileS158059557) call() {

	if sigRecv.err != nil {
		sigRecv.t.
			Fatalf("marshaling the golden Report must not fail: %v", sigRecv.err)
	}
}

type sigTestReportMarshalMatchesGoldenFileS455697778 struct {
	err error
	t   *testing.
		T
}

func (sigRecv *sigTestReportMarshalMatchesGoldenFileS455697778) call() {

	if sigRecv.err != nil {
		sigRecv.t.
			Fatalf("reading testdata/report_golden.json must not fail: %v", sigRecv.err)
	}
}

type sigTestReportMarshalMatchesGoldenFileS561530703 struct {
	got []byte
	t   *testing.
		T
	want []byte
}

func (sigRecv *sigTestReportMarshalMatchesGoldenFileS561530703) call() {

	if string(sigRecv.got) != string(sigRecv.want) {
		sigRecv.t.
			Errorf("Report JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", sigRecv.got, sigRecv.want)
	}
}

type sigTestReportMarshalMatchesGoldenFileS761534661 struct {
	roundTripped *Report
	t            *testing.
			T
	want []byte
}

func (sigRecv *sigTestReportMarshalMatchesGoldenFileS761534661) call() {

	if err := json.Unmarshal(sigRecv.want, sigRecv.roundTripped); err != nil {
		sigRecv.t.
			Fatalf("golden file must unmarshal back into a Report: %v", err)
	}
}

type sigTestReportMarshalMatchesGoldenFileS855176925 struct {
	roundTripped Report
	t            *testing.
			T
}

func (sigRecv *sigTestReportMarshalMatchesGoldenFileS855176925) call() {

	if sigRecv.roundTripped.ReportVersion != ReportVersion1 {
		sigRecv.t.
			Errorf("Report.report_version: got %q, want %q", sigRecv.roundTripped.ReportVersion, ReportVersion1)
	}
}
