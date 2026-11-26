package evalpatterns

const (
	Complete       = 0
	InputInvalid   = 2
	AnalysisFailed = 3
)

type Manifest struct {
	SchemaVersion string      `json:"schema_version"`
	References    []Reference `json:"references"`
}

type Reference struct {
	Label    string `json:"label"`
	SHA      string `json:"sha"`
	Language string `json:"language"`
	Licence  string `json:"licence"`
}

type Refusal struct {
	Label  string
	Exit   int
	Reason string
}

func (r Refusal) Error() string { return "" }

func LoadManifest(path string) (Manifest, error) { return Manifest{}, nil }

func (r Reference) Check() error { return nil }

func CheckoutReference(reference Reference, clone, dir string) error { return nil }

func ValidateProfile(raw []byte) error { return nil }
