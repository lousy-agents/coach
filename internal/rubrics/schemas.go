package rubrics

import _ "embed"

//go:embed schemas/hidden_mutation_contextualization_v1.json
var schemaHiddenMutationV1 []byte

//go:embed schemas/change_cohesion_v1.json
var schemaChangeCohesionV1 []byte
