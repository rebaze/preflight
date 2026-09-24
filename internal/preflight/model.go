package preflight

// InitOptions selects the baseline and local policy explicitly trusted by init.
type InitOptions struct{ Repo, Baseline, ProfilePath, PolicyDir, StateDir, ConftestPath string }
type Options struct {
	StateDir, Base string
	All            bool
}

type Finding struct {
	ControlID   string       `json:"controlId"`
	Status      string       `json:"status"`
	ReasonCode  string       `json:"reasonCode"`
	Message     string       `json:"message"`
	Paths       []string     `json:"paths"`
	Evidence    []Evidence   `json:"evidence"`
	Owner       string       `json:"owner"`
	NextActions []NextAction `json:"nextActions"`
}
type Evidence struct {
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
	Path   string `json:"path"`
}
type NextAction struct {
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
}
type Subject struct {
	RepoRoot           string   `json:"repoRoot"`
	BaselineCommit     string   `json:"baselineCommit"`
	ComparisonBase     string   `json:"comparisonBase"`
	Head               string   `json:"head"`
	MergeBase          string   `json:"mergeBase"`
	SnapshotDigest     string   `json:"snapshotDigest"`
	ExcludedCategories []string `json:"excludedCategories"`
	ChangedPaths       []string `json:"changedPaths"`
	OutOfScopePaths    []string `json:"outOfScopePaths"`
}
type Policy struct {
	PackageDigest  string `json:"packageDigest"`
	ProfileDigest  string `json:"profileDigest"`
	BaselineCommit string `json:"baselineCommit"`
}
type Runtime struct {
	EvaluatorDigest        string `json:"evaluatorDigest"`
	EvaluatorVersion       string `json:"evaluatorVersion"`
	RunnerImageID          string `json:"runnerImageId"`
	PreparationInputDigest string `json:"preparationInputDigest"`
}
type Scope struct {
	Profile  string   `json:"profile"`
	Deferred []string `json:"deferred"`
	Excluded []string `json:"excluded"`
}
type Summary struct {
	Pass           int `json:"pass"`
	Fail           int `json:"fail"`
	Missing        int `json:"missing"`
	Error          int `json:"error"`
	ReviewRequired int `json:"review_required"`
	Deferred       int `json:"deferred"`
	NotApplicable  int `json:"not_applicable"`
}
type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	Mode          string    `json:"mode"`
	Authority     string    `json:"authority"`
	RunID         string    `json:"runId"`
	StartedAt     string    `json:"startedAt"`
	FinishedAt    string    `json:"finishedAt"`
	Current       bool      `json:"current"`
	Subject       Subject   `json:"subject"`
	Policy        Policy    `json:"policy"`
	Runtime       Runtime   `json:"runtime"`
	Scope         Scope     `json:"scope"`
	Findings      []Finding `json:"findings"`
	Summary       Summary   `json:"summary"`
	ExitCode      int       `json:"exitCode"`
}

func NewFinding(control, status, reason, message string) Finding {
	return Finding{ControlID: control, Status: status, ReasonCode: reason, Message: message, Paths: []string{}, Evidence: []Evidence{}, NextActions: []NextAction{}}
}
func validStatus(s string) bool {
	switch s {
	case "pass", "fail", "missing", "error", "review_required", "deferred", "not_applicable":
		return true
	}
	return false
}

// ExitCode preserves all outcomes, giving errors and missing evidence precedence.
func ExitCode(findings []Finding) int {
	code := 0
	for _, f := range findings {
		n := 0
		switch f.Status {
		case "error":
			n = 3
		case "missing":
			n = 2
		case "fail":
			n = 1
		default:
			if !validStatus(f.Status) {
				n = 3
			}
		}
		if n > code {
			code = n
		}
	}
	return code
}
func summarize(findings []Finding) Summary {
	s := Summary{}
	for _, f := range findings {
		switch f.Status {
		case "pass":
			s.Pass++
		case "fail":
			s.Fail++
		case "missing":
			s.Missing++
		case "error":
			s.Error++
		case "review_required":
			s.ReviewRequired++
		case "deferred":
			s.Deferred++
		case "not_applicable":
			s.NotApplicable++
		}
	}
	return s
}

// FinalizeReport fills shape defaults and derives counts/exit status from the findings.
// It deliberately does not create findings or invent missing subject identities.
func FinalizeReport(r *Report) {
	r.SchemaVersion = 1
	r.Authority = "local-feedback"
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if r.Subject.ExcludedCategories == nil {
		r.Subject.ExcludedCategories = []string{}
	}
	if r.Subject.ChangedPaths == nil {
		r.Subject.ChangedPaths = []string{}
	}
	if r.Subject.OutOfScopePaths == nil {
		r.Subject.OutOfScopePaths = []string{}
	}
	if r.Scope.Deferred == nil {
		r.Scope.Deferred = []string{}
	}
	if r.Scope.Excluded == nil {
		r.Scope.Excluded = []string{}
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		if f.Paths == nil {
			f.Paths = []string{}
		}
		if f.Evidence == nil {
			f.Evidence = []Evidence{}
		}
		if f.NextActions == nil {
			f.NextActions = []NextAction{}
		}
		for j := range f.NextActions {
			if f.NextActions[j].Args == nil {
				f.NextActions[j].Args = []string{}
			}
		}
	}
	r.Summary = summarize(r.Findings)
	r.ExitCode = ExitCode(r.Findings)
	if r.Mode == "status" && !r.Current && r.ExitCode < 2 {
		r.ExitCode = 2
	}
}
