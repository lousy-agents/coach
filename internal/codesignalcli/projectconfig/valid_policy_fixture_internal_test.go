package projectconfig

// validPolicy is the smallest committed policy the schema accepts. A restore is offered only for a
// committed copy that is valid, so a test that expects one commits this and not an empty object.
const validPolicy = `{"schema_version":"1","roots":["."]}`
