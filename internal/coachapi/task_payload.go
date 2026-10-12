package coachapi

import (
	"encoding/json"
)

// TaskPayloadSchemaVersion1 is the supported queue.Task.Payload schema
// version for this package. ADR-006 requires versioned queue payloads; the
// worker decodes this wire shape independently of the job row.
const TaskPayloadSchemaVersion1 = 1

// taskPayload is the opaque queue.Task.Payload wire shape this package
// enqueues. The worker re-reads the job from the store, so only the
// schema version and job id travel through the queue. Task.ID remains the
// idempotency key; the queue adapter must not interpret these fields.
type taskPayload struct {
	SchemaVersion int    `json:"schema_version"`
	JobID         string `json:"job_id"`
}

// MarshalTaskPayload returns the ADR-006 versioned queue.Task.Payload body for
// jobID. POST /v1/jobs and the worker requeue reconciler must use this helper
// so submit and recovery publish the same wire shape.
func MarshalTaskPayload(jobID string) ([]byte, error) {
	return json.Marshal(taskPayload{
		SchemaVersion: TaskPayloadSchemaVersion1,
		JobID:         jobID,
	})
}
