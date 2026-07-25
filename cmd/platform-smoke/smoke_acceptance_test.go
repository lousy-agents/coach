package main

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("platform-smoke submit status contract", func() {
	When("POST /v1/jobs returns 202 Accepted with a job id body", func() {
		It("accepts the status so smoke can continue polling", func() {
			Expect(checkSubmitStatus(http.StatusAccepted, []byte(`{"id":"job-1"}`))).To(Succeed())
		})
	})

	When("POST /v1/jobs returns 200 OK", func() {
		It("fails so a submit-status regression cannot pass CI smoke", func() {
			err := checkSubmitStatus(http.StatusOK, []byte(`{"id":"job-1"}`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("want 202"))
		})
	})

	When("POST /v1/jobs returns a non-success status", func() {
		It("fails with the status code in the error", func() {
			err := checkSubmitStatus(http.StatusInternalServerError, []byte(`{"error":{}}`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("500"))
		})
	})
})

var _ = Describe("platform-smoke report provenance contract", func() {
	const jobID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	When("the report has report_version 1, matching job metadata, and both provenance sources", func() {
		It("accepts the report", func() {
			body := []byte(`{
				"report_version": "1",
				"job_id": "` + jobID + `",
				"kind": "repo_baseline_scan",
				"findings": [
					{"source": "deterministic"},
					{"source": "agent"}
				],
				"error": null
			}`)
			Expect(validateReportBody(body, jobID)).To(Succeed())
		})
	})

	When("the report lacks source=deterministic findings", func() {
		It("fails so a no-op deterministic path cannot pass smoke", func() {
			body := []byte(`{
				"report_version": "1",
				"job_id": "` + jobID + `",
				"kind": "repo_baseline_scan",
				"findings": [{"source": "agent"}]
			}`)
			err := validateReportBody(body, jobID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("source=deterministic"))
		})
	})

	When("the report lacks source=agent findings", func() {
		It("fails so a broken stub/gateway path cannot pass smoke", func() {
			body := []byte(`{
				"report_version": "1",
				"job_id": "` + jobID + `",
				"kind": "repo_baseline_scan",
				"findings": [{"source": "deterministic"}]
			}`)
			err := validateReportBody(body, jobID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("source=agent"))
		})
	})

	When("the report job_id does not match the submitted job", func() {
		It("fails closed", func() {
			body := []byte(`{
				"report_version": "1",
				"job_id": "other-id",
				"kind": "repo_baseline_scan",
				"findings": [
					{"source": "deterministic"},
					{"source": "agent"}
				]
			}`)
			err := validateReportBody(body, jobID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("job_id"))
		})
	})
})
